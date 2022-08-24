package storage

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dreitier/backmon/backup"
	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/metrics"
	fs "github.com/dreitier/backmon/storage/fs"
	log "github.com/sirupsen/logrus"
)

var (
	clients    = make(map[string]*clientData)
	mutex      = &sync.Mutex{}
	ignoreFile = &fs.FileInfo{Name: ".backmonignore"}
)

func InitializeConfiguration() {
	envs := config.GetInstance().Environments()
	for _, env := range envs {
		clients[env.Name] = &clientData{
			DefinitionFilename: env.Definitions,
			Definition:         &fs.FileInfo{Name: env.Definitions},
			Client:             NewClient(env.Client),
			Disks:              make(map[string]*DiskData),
		}
	}
}

type clientData struct {
	DefinitionFilename string
	Definition         *fs.FileInfo
	Client             Client
	Disks              map[string]*DiskData
}

func (client *clientData) updateDiskInfo() error {
	diskNames, err := client.Client.GetDiskNames()
	if err != nil {
		client.dropAllDisks()
		return err
	}

	// find all disks that were removed on the client
	var removed []string
	for oldDisk := range client.Disks {
		exists := false
		for _, newDisk := range diskNames {
			if newDisk == oldDisk {
				exists = true
				break
			}
		}
		if !exists {
			removed = append(removed, oldDisk)
		}
	}
	// delete the removed disks from the map
	for _, removeDisk := range removed {
		client.Disks[removeDisk].metrics.Drop()
		delete(client.Disks, removeDisk)
	}

	// add disks that are new on the client to the map
	for _, diskName := range diskNames {
		if !config.GetInstance().Disks().IsDiskIncluded(diskName) {
			continue
		}

		_, exists := client.Disks[diskName]

		// ignore disks containing a file called '.backmonignore'
		buf, err := client.Client.Download(diskName, ignoreFile)
		if err == nil {
			// .backmonignore found
			_ = buf.Close()
			if exists {
				client.Disks[diskName].metrics.Drop()
				delete(client.Disks, diskName)
			}
			log.Infof("Found file '%s' in disk %s, ignoring disk.", ignoreFile.Name, diskName)
			continue
		}

		if !exists {
			safeAlias, _ := backup.MakeLegalAlias(diskName)
			// no warning for now to avoid spamming the logs on every refresh
			// if warn {
			//	log.Warnf("The disk '%s' contained non-url characters, its name will be '%s' in urls", diskName, safeAlias)
			// }

			client.Disks[diskName] = &DiskData{
				Name:     diskName,
				SafeName: safeAlias,
				metrics:  metrics.NewDisk(diskName),
			}
		}
	}
	return nil
}

func (client *clientData) dropAllDisks() {
	for _, disk := range client.Disks {
		disk.metrics.Drop()
	}
	client.Disks = make(map[string]*DiskData)
}

type DiskData struct {
	Name            string
	SafeName        string
	metrics         *metrics.DiskMetric
	groups          []map[string][]*fs.FileInfo
	Definition      backup.Definition
	definitionsHash [sha1.Size]byte
}

func (disk *DiskData) MarshalJSON() ([]byte, error) {
	return json.Marshal(disk.Name)
}

func (disk *DiskData) updateDefinitions(data io.Reader) {
	var buf bytes.Buffer

	duplicate := io.TeeReader(data, &buf)
	changed, err := disk.hashChanged(duplicate)
	if err != nil {
		log.Errorf("Failed to update backup definitions in '%s': %s", disk.Name, err)
		disk.Definition = nil
		disk.metrics.DefinitionsMissing()
		return
	}

	if !changed {
		log.Debugf("Backup definitions in '%s' are unchanged.", disk.Name)
		return
	}

	log.Infof("Backup definitions in '%s' changed, parsing new definitions.", disk.Name)
	disk.Definition, err = backup.ParseDefinition(&buf)
	if err != nil {
		log.Errorf("Failed to parse backup definitions in '%s': %s", disk.Name, err)
		disk.metrics.DefinitionsMissing()
		return
	}

	disk.metrics.DefinitionsUpdated()
	disk.groups = make([]map[string][]*fs.FileInfo, len(disk.Definition))
}

func (disk *DiskData) hashChanged(data io.Reader) (changed bool, err error) {
	sha := sha1.New()
	count, err := io.Copy(sha, data)
	if err != nil {
		return false, fmt.Errorf("failed to compute file hash: %s", err)
	}
	_ = count
	hash := make([]byte, 0, sha1.Size)
	hash = sha.Sum(hash)
	for i := 0; i < sha1.Size; i++ {
		if disk.definitionsHash[i] != hash[i] {
			changed = true
			break
		}
	}
	if changed {
		for i := 0; i < sha1.Size; i++ {
			disk.definitionsHash[i] = hash[i]
		}
	}
	return changed, nil
}

func (disk *DiskData) maxDepth() uint {
	maxDepth := uint(0)
	for _, dir := range disk.Definition {
		depth := uint(len(dir.Filter.Layers))
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

type TemporalFile struct {
	Time time.Time
	File *fs.FileInfo
}

type FileGroup []TemporalFile
type FileLookup map[string][]FileGroup

// BEGIN The following Methods for FileGroup have to be implemented for sort.Interface
// Len is the number of elements in the collection.
func (list FileGroup) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list FileGroup) Less(i int, j int) bool {
	return list[i].Time.After(list[j].Time)
}

// Swap swaps the elements with indexes i and j.
func (list FileGroup) Swap(i int, j int) {
	list[i], list[j] = list[j], list[i]
}

// END

func (list FileGroup) Purge(fileDef *backup.BackupFileDefinition, path string, disk string, client Client) (remainder FileGroup, young uint64) {
	threshold := time.Now().UTC().Add(-fileDef.RetentionAge)
	young = uint64(sort.Search(len(list), func(i int) bool { return list[i].Time.Before(threshold) }))

	keep := fileDef.RetentionCount
	if young > keep {
		keep = young
	}

	if !fileDef.Purge || uint64(len(list)) <= keep {
		return list, young
	}

	excess := list[keep:]
	log.Infof("Purging %d excess files matching %#q in %#q from disk %#q", len(excess), fileDef.Pattern, path, disk)

	for _, file := range excess {
		err := client.Delete(disk, file.File)

		if err != nil {
			list[keep] = file
			keep++
			log.Warnf("Could not purge file '%s': %s", file.File.Name, err)
		} else {
			log.Infof("Purged file '%s'", file.File.Name)
		}
	}
	return list[:keep], young
}

func UpdateDiskInfo() {
	log.Info("Updating disks info...")
	mutex.Lock()
	defer mutex.Unlock()

	for environmentName, client := range clients {
		log.Debugf("[env:%s] Updating disks", environmentName)

		if err := client.updateDiskInfo(); err != nil {
			log.Errorf("[env:%s] Could not retrieve disk names from client: %v", environmentName, err)
			continue
		}

		for diskName, disk := range client.Disks {
			log.Debugf("[env:%s][disk:%s] Downloading backup definitions file", environmentName, diskName)

			buf, err := client.Client.Download(diskName, client.Definition)
			if err != nil {
				log.Errorf("[env:%s][disk:%s] Backup definitions file '%s' could not be opened: %v", environmentName, diskName, client.DefinitionFilename, err)
				disk.metrics.DefinitionsMissing()
				continue
			}

			disk.updateDefinitions(buf)
			_ = buf.Close()

			files, err := client.Client.GetFileNames(diskName, disk.maxDepth())
			if err != nil {
				log.Errorf("[env:%s][disk:%s] Failed to retrieve files from disk: %v", environmentName, diskName, err)
				// don't just return, we still need to update the metrics!
				files = &fs.DirectoryInfo{Name: diskName}
			}

			updateMetrics(client.Client, disk, files)
		}
	}

	log.Debug("... disks info updated")
}

func updateMetrics(client Client, disk *DiskData, root *fs.DirectoryInfo) {
	log.Debugf("Updating metrics ...")

	now := time.Now()
	for iDir, dirDef := range disk.Definition {
		log.Debugf("# %s", dirDef.Alias)
		vars := make([]string, len(dirDef.Filter.Variables))
		fileGroups := make(FileLookup, len(dirDef.Files))
		findMatchingDirs(root, dirDef, 0, 0, vars, fileGroups)

		if len(fileGroups) == 0 {
			log.Warnf("Could not find any file groups. Either the root directory is wrong or no files are matching the defined pattern")
		}

		for _, fileDef := range dirDef.Files {
			lastRun := backup.FindPrevious(fileDef.Schedule, now)
			disk.metrics.UpdateFileLimits(dirDef.Alias, fileDef.Alias, fileDef.RetentionCount, fileDef.RetentionAge, lastRun)
		}

		currentGroups := make(map[string][]*fs.FileInfo, len(fileGroups))

		for group, fileMatches := range fileGroups {
			latest := make([]*fs.FileInfo, len(dirDef.Files))

			for k, fileDef := range dirDef.Files {
				matches := fileMatches[k]
				sort.Sort(matches)
				matches, young := matches.Purge(fileDef, group, disk.Name, client)

				disk.metrics.UpdateFileCounts(dirDef.Alias, fileDef.Alias, group, len(matches), young)

				if len(matches) > 0 {
					latest[k] = matches[0].File

					log.Debugf("      > %s < selected as latest/newest file based upon sorting algorithm", matches[0].File.Name)

					disk.metrics.UpdateLatestFile(
						dirDef.Alias,
						fileDef.Alias,
						group,
						matches[0].File,
						matches[0].Time)
				}
			}

			currentGroups[group] = latest
		}

		pastGroups := disk.groups[iDir]
		disk.groups[iDir] = currentGroups

		for group := range pastGroups {
			if _, exists := fileGroups[group]; exists {
				continue
			}

			for _, fileDef := range dirDef.Files {
				disk.metrics.DropFile(dirDef.Alias, fileDef.Alias, group)
			}
		}
	}
}

func findMatchingDirs(
	dir *fs.DirectoryInfo,
	dirDef *backup.Directory,
	level uint,
	offset uint,
	vars []string,
	fileGroups FileLookup,
) {
	if level >= uint(len(dirDef.Filter.Layers)) {
		//Matching directory reached
		path := assembleFromTemplate(dirDef.Filter.Template, dirDef.Filter.Variables, vars)
		//TODO: collect variable values
		matches := findMatchingFiles(dir, dirDef, vars)
		group, exists := fileGroups[path]

		if !exists {
			group = make([]FileGroup, len(dirDef.Files))
			fileGroups[path] = group
		}

		for i := 0; i < len(group); i++ {
			group[i] = append(group[i], matches[i]...)
		}

		return
	}

	pattern := dirDef.Filter.Layers[level]

	for _, subDir := range dir.SubDirs {
		match := pattern.FindStringSubmatch(subDir.Name)

		if match == nil {
			continue
		}

		logDir(level, subDir.Name)

		for i, capture := range match {
			if i > 0 {
				vars[offset+uint(i)-1] = capture
			}
		}

		findMatchingDirs(subDir, dirDef, level+1, offset+uint(len(match)-1), vars, fileGroups)
	}
}

func logDir(level uint, dir string) {
	str := strings.Builder{}
	str.Grow(6 + 2*int(level) + len(dir))
	str.WriteString("  > ")

	for i := 0; i < int(level); i++ {
		str.WriteString("./") //…
	}

	str.WriteString(dir)
	log.Debug(str.String())
}

func findMatchingFiles(
	dir *fs.DirectoryInfo,
	dirDef *backup.Directory,
	vars []string,
) []FileGroup {
	timestamp := timestampFromVars(dirDef.Filter.Variables, vars)
	fileGroup := make([]FileGroup, len(dirDef.Files))
	var matches FileGroup

	for i, fileDef := range dirDef.Files {
		matches = matches[:0]
		log.Debugf("    ~ %s", fileDef.Alias)
		matches = collectMatchingFiles(dir.Files, fileDef, vars, &timestamp, matches)
		fileGroup[i] = append(fileGroup[i], matches...)
	}

	return fileGroup
}

func timestampFromVars(varDefs []backup.VariableDefinition, varVals []string) backup.Timestamp {
	timestamp := backup.Timestamp{}

	//Iterate over the variables in reverse order so that earlier
	// occurrences of a time substitution override later ones
	for i := len(varDefs) - 1; i >= 0; i-- {
		if varDefs[i].Parser != nil {
			varDefs[i].Parser(varVals[i], &timestamp)
		}
	}

	return timestamp
}

func assembleFromTemplate(template []string, varDefs []backup.VariableDefinition, vars []string) string {
	if len(template) == 0 {
		return "."
	}

	str := strings.Builder{}

	for i, v := range varDefs {
		str.WriteString(template[i])

		if !v.Fuse {
			str.WriteString(vars[i])
			continue
		}

		if v.Name[0] == backup.SubstitutionMarker {
			str.WriteString(v.Name)
			continue
		}

		str.WriteString("{{")
		str.WriteString(v.Name)
		str.WriteString("}}")
	}

	str.WriteString(template[len(template)-1])

	return str.String()
}

func collectMatchingFiles(
	files []*fs.FileInfo,
	fileDef *backup.BackupFileDefinition,
	vars []string,
	folderTime *backup.Timestamp,
	matches FileGroup,
) FileGroup {
	for _, file := range files {
		match := fileDef.Filter.FindStringSubmatch(file.Name)
		if match == nil {
			continue
		}

		timestamp := *folderTime
		matchingVars := true

		for k, capture := range match {
			varMap := fileDef.VariableMapping[k]

			if varMap.Offset == 0 {
				//CaptureGroup refers to an internal variable
				if varMap.Parser != nil {
					varMap.Parser(capture, &timestamp)
				}

				continue
			}

			//CaptureGroup refers to a user-defined variable
			value := vars[varMap.Offset-1]

			if varMap.Conversion != nil {
				//Apply conversion function to variable value
				value = varMap.Conversion(value)
			}

			if capture != value {
				matchingVars = false
				break
			}
		}

		if matchingVars {
			var sortByTime *time.Time
			var useDefaultsFromTime *time.Time

			// first of, we have to identify which file attribute to use as a baseline for interpolated timestamps
			switch fileDef.SortBy {
			case backup.SORT_BY_BORN_AT:
				useDefaultsFromTime = &file.BornAt
				break
			case backup.SORT_BY_ARCHIVED_AT:
				useDefaultsFromTime = &file.ArchivedAt
				break
			// default includes "interpolation": we can't reference an interpolation time as default because it's not assigned yet
			default:
				useDefaultsFromTime = &file.ModifiedAt
			}

			// keep the interpolated timestamp in its own variable to make go happy
			interpolatedTimestamp := timestamp.TimeWithDefaults(*useDefaultsFromTime)

			// set the file's interpolated timestamp
			file.InterpolatedTimestamp = &interpolatedTimestamp

			switch fileDef.SortBy {
			case backup.SORT_BY_BORN_AT:
				sortByTime = &file.BornAt
				break
			case backup.SORT_BY_MODIFIED_AT:
				sortByTime = &file.ModifiedAt
				break
			case backup.SORT_BY_ARCHIVED_AT:
				sortByTime = &file.ArchivedAt
				break
			// by default we are using the interpolated timestamp
			default:
				sortByTime = file.InterpolatedTimestamp
			}

			// [:19] chops off timezone information, which is always ' +0000 UTC'
			log.Debugf("      - %s @ %s | born:%s | mod:%s | arch:%s | interpolated:%s",
				file.Name,
				sortByTime.String()[:19],
				file.BornAt.String()[:19],
				file.ModifiedAt.String()[:19],
				file.ArchivedAt.String()[:19],
				file.InterpolatedTimestamp.String()[:19])

			matches = append(matches, TemporalFile{*sortByTime, file})
		}
	}

	return matches
}

func GetDisks() []*DiskData {
	total := 0

	for _, client := range clients {
		total += len(client.Disks)
	}

	disks := make([]*DiskData, 0, total)

	for _, client := range clients {
		for _, disk := range client.Disks {
			disks = append(disks, disk)
		}
	}

	return disks
}

func GetFilenames(
	diskName string,
	directoryName string,
	fileName string,
) []string {
	groups, file := findGroups(diskName, directoryName, fileName)
	if groups == nil {
		return nil
	}

	results := make([]string, 0, len(groups))

	for groupName, files := range groups {
		if files[file] != nil {
			results = append(results, groupName)
		}
	}

	return results
}

func Download(
	diskName string,
	directoryName string,
	fileName string,
	groupName string,
) (bytes io.ReadCloser, err error) {
	groups, file := findGroups(diskName, directoryName, fileName)

	if groups == nil {
		return nil, errors.New("the requested file does not exist")
	}

	var client *clientData

	for _, client = range clients {
		if _, found := client.Disks[diskName]; found {
			break
		}
	}

	if client == nil {
		return nil, errors.New("the requested file does not exist")
	}

	return client.Client.Download(diskName, groups[groupName][file])
}

func findGroups(
	diskName string,
	directoryName string,
	fileName string,
) (map[string][]*fs.FileInfo, int) {
	disk := FindDisk(diskName)

	if disk == nil {
		return nil, 0
	}

	var dirI int

	for dirI = 0; dirI < len(disk.Definition); dirI++ {
		if disk.Definition[dirI].Alias == directoryName {
			break
		}
	}

	if dirI >= len(disk.Definition) {
		return nil, 0
	}

	dir := disk.Definition[dirI]
	var fileI int

	for fileI = 0; fileI < len(dir.Files); fileI++ {
		if dir.Files[fileI].Alias == fileName {
			return disk.groups[dirI], fileI
		}
	}

	return nil, 0
}

func FindDisk(diskName string) *DiskData {
	for _, client := range clients {
		if disk, found := client.Disks[diskName]; found {
			return disk
		}
	}

	return nil
}

func FindDirectory(
	diskName string,
	directoryName string,
) *backup.Directory {
	disk := FindDisk(diskName)

	if disk == nil {
		return nil
	}

	for _, dir := range disk.Definition {
		if dir.Alias == directoryName {
			return dir
		}
	}

	return nil
}
