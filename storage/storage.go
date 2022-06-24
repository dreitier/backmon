package storage

import (
	"bytes"
	"github.com/dreitier/cloudmon/backup"
	"github.com/dreitier/cloudmon/config"
	"github.com/dreitier/cloudmon/metrics"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	clients = make(map[string]*clientData)
	mutex   = &sync.Mutex{}
	ignoreFile = &FileInfo{Name:".cloudmonignore"}
)

func init() {
	envs := config.GetInstance().Environments()
	for _, env := range envs {
		clients[env.Name] = &clientData{
			DefinitionFilename: env.Definitions,
			Definition:         &FileInfo{Name:env.Definitions},
			Client:             NewClient(env.Client),
			Buckets:            make(map[string]*BucketData),
		}
	}
}

type clientData struct {
	DefinitionFilename string
	Definition         *FileInfo
	Client             Client
	Buckets            map[string]*BucketData
}

func (client *clientData) updateBucketInfo() error {
	bucketNames, err := client.Client.GetBucketNames()
	if err != nil {
		client.dropAllBuckets()
		return err
	}

	// find all buckets that were removed on the client
	var removed []string
	for oldBucket := range client.Buckets {
		exists := false
		for _, newBucket := range bucketNames {
			if newBucket == oldBucket {
				exists = true
				break
			}
		}
		if !exists {
			removed = append(removed, oldBucket)
		}
	}
	// delete the removed buckets from the map
	for _, removeBucket := range removed {
		client.Buckets[removeBucket].metrics.Drop()
		delete(client.Buckets, removeBucket)
	}

	// add buckets that are new on the client to the map
	for _, bucketName := range bucketNames {
		if config.GetInstance().Global().IgnoreBucket(bucketName) {
			continue
		}
		_, exists := client.Buckets[bucketName]

		// ignore buckets containing a file called '.cloudmonignore'
		buf, err := client.Client.Download(bucketName, ignoreFile)
		if err == nil {
			// .cloudmonignore found
			_ = buf.Close()
			if exists {
				client.Buckets[bucketName].metrics.Drop()
				delete(client.Buckets, bucketName)
			}
			log.Debugf("Found file '%s' in bucket %s, ignoring bucket.", ignoreFile.Name, bucketName)
			continue
		}

		if !exists {
			safeAlias, _ := backup.MakeLegalAlias(bucketName)
			// no warning for now to avoid spamming the logs on every refresh
			// if warn {
			//	log.Warnf("The bucket '%s' contained non-url characters, its name will be '%s' in urls", bucketName, safeAlias)
			// }

			client.Buckets[bucketName] = &BucketData{
				Name:     bucketName,
				SafeName: safeAlias,
				metrics:  metrics.NewBucket(bucketName),
			}
		}
	}
	return nil
}

func (client *clientData) dropAllBuckets() {
	for _, bucket := range client.Buckets {
		bucket.metrics.Drop()
	}
	client.Buckets = make(map[string]*BucketData)
}

type BucketData struct {
	Name            string
	SafeName        string
	metrics         *metrics.Bucket
	groups          []map[string][]*FileInfo
	Definition      backup.Definition
	definitionsHash [sha1.Size]byte
}

func (bucket *BucketData)  MarshalJSON() ([]byte, error) {
	return json.Marshal(bucket.Name)
}

func (bucket *BucketData) updateDefinitions(data io.Reader) {
	var buf bytes.Buffer
	duplicate := io.TeeReader(data, &buf)
	changed, err := bucket.hashChanged(duplicate)
	if err != nil {
		log.Errorf("Failed to update backup definitions in '%s': %s", bucket.Name, err)
		bucket.Definition = nil
		bucket.metrics.DefinitionsMissing()
		return
	}
	if !changed {
		log.Debugf("Backup definitions in '%s' are unchanged.", bucket.Name)
		return
	}
	log.Infof("Backup definitions in '%s' changed, parsing new definitions.", bucket.Name)
	bucket.Definition, err = backup.ParseDefinition(&buf)
	if err != nil {
		log.Errorf("Failed to parse backup definitions in '%s': ", bucket.Name, err)
		bucket.metrics.DefinitionsMissing()
		return
	}
	bucket.metrics.DefinitionsUpdated()
	bucket.groups = make([]map[string][]*FileInfo, len(bucket.Definition))
}

func (bucket *BucketData) hashChanged(data io.Reader) (changed bool, err error) {
	sha := sha1.New()
	count, err := io.Copy(sha, data)
	if err != nil {
		return false, fmt.Errorf("failed to compute file hash: %s", err)
	}
	_ = count
	hash := make([]byte, 0, sha1.Size)
	hash = sha.Sum(hash)
	for i := 0; i < sha1.Size; i++ {
		if bucket.definitionsHash[i] != hash[i] {
			changed = true
			break
		}
	}
	if changed {
		for i := 0; i < sha1.Size; i++ {
			bucket.definitionsHash[i] = hash[i]
		}
	}
	return changed, nil
}

func (bucket *BucketData) maxDepth() uint {
	maxDepth := uint(0)
	for _, dir := range bucket.Definition {
		depth := uint(len(dir.Filter.Layers))
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

type TemporalFile struct {
	Time time.Time
	File *FileInfo
}

type FileGroup []TemporalFile
type FileLookup map[string][]FileGroup

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

func (list FileGroup) Purge(fileDef *backup.File, path string, bucket string, client Client) (remainder FileGroup, young uint64) {
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
	log.Infof("Purging %d excess files matching %#q in %#q from bucket %#q", len(excess), fileDef.Pattern, path, bucket)
	for _, file := range excess {
		err := client.Delete(bucket, file.File)
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

func UpdateBucketInfo() {
	log.Info("Updating bucket info...")
	mutex.Lock()
	defer mutex.Unlock()

	for clientName, client := range clients {
		log.Debugf("[Client] %s -> %s", clientName, client.DefinitionFilename)
		if err := client.updateBucketInfo(); err != nil {
			log.Errorf("Could not retrieve bucket names from client %s: %v", clientName, err)
			continue
		}

		for bucketName, bucket := range client.Buckets {
			log.Debugf("[Bucket] %s :", bucketName)
			buf, err := client.Client.Download(bucketName, client.Definition)
			if err != nil {
				log.Errorf("Backup definitions file '%s' in bucket %s could not be opened: %v", client.DefinitionFilename, bucketName, err)
				bucket.metrics.DefinitionsMissing()
				continue
			}
			bucket.updateDefinitions(buf)
			_ = buf.Close()

			files, err := client.Client.GetFileNames(bucketName, bucket.maxDepth())
			if err != nil {
				log.Errorf("Failed to retrieve files from bucket %s: %v", bucketName, err)
				//Don't just return, we still need to update the metrics!
				files = &DirectoryInfo{Name: bucketName}
			}

			updateMetrics(client.Client, bucket, files)
		}
	}
	log.Debug("...Bucket info updated")
}

func updateMetrics(client Client, bucket *BucketData, root *DirectoryInfo) {
	now := time.Now()
	for iDir, dirDef := range bucket.Definition {
		log.Debugf("# %s", dirDef.Alias)
		vars := make([]string, len(dirDef.Filter.Variables))
		fileGroups := make(FileLookup, len(dirDef.Files))
		findMatchingDirs(root, dirDef, 0, 0, vars, fileGroups)

		for _, fileDef := range dirDef.Files {
			lastRun := backup.FindPrevious(fileDef.Schedule, now)
			bucket.metrics.FileLimits(dirDef.Alias, fileDef.Alias, fileDef.RetentionCount, fileDef.RetentionAge, lastRun)
		}

		currentGroups := make(map[string][]*FileInfo, len(fileGroups))
		for group, fileMatches := range fileGroups {
			latest := make([]*FileInfo, len(dirDef.Files))
			for k, fileDef := range dirDef.Files {
				matches := fileMatches[k]
				sort.Sort(matches)
				matches, young := matches.Purge(fileDef, group, bucket.Name, client)

				bucket.metrics.FileCounts(dirDef.Alias, fileDef.Alias, group, len(matches), young)
				if len(matches) > 0 {
					latest[k] = matches[0].File
					bucket.metrics.LatestFile(dirDef.Alias, fileDef.Alias, group, matches[0].File.Size, matches[0].Time)
				}
			}
			currentGroups[group] = latest
		}

		pastGroups := bucket.groups[iDir]
		bucket.groups[iDir] = currentGroups
		for group := range pastGroups {
			if _, exists := fileGroups[group]; exists {
				continue
			}
			for _, fileDef := range dirDef.Files {
				bucket.metrics.DropFile(dirDef.Alias, fileDef.Alias, group)
			}
		}
	}
}

func findMatchingDirs(
	dir *DirectoryInfo,
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
	dir *DirectoryInfo,
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
		if v.Fuse {
			if v.Name[0] == backup.SubstitutionMarker {
				str.WriteString(v.Name)
			} else {
				str.WriteString("{{")
				str.WriteString(v.Name)
				str.WriteString("}}")
			}
		} else {
			str.WriteString(vars[i])
		}
	}
	str.WriteString(template[len(template)-1])
	return str.String()
}

func collectMatchingFiles(
	files []*FileInfo,
	fileDef *backup.File,
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
			//TODO: use the timing method that was selected in the definitions file
			fileTime := timestamp.TimeWithDefaults(file.Timestamp)
			//[:19] chops off timezone information, which is always ' +0000 UTC'
			log.Debugf("      - %s @ %s", file.Name, fileTime.String()[:19])
			matches = append(matches, TemporalFile{fileTime, file})
		}
	}
	return matches
}

func GetBuckets() []*BucketData {
	total := 0
	for _, client := range clients {
		total += len(client.Buckets)
	}
	buckets := make([]*BucketData, 0, total)
	for _, client := range clients {
		for _, bucket := range client.Buckets {
			buckets = append(buckets, bucket)
		}
	}
	return buckets
}

func GetFilenames(
	bucketName string,
	directoryName string,
	fileName string,
) []string {
	groups, file := findGroups(bucketName, directoryName, fileName)
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
	bucketName string,
	directoryName string,
	fileName string,
	groupName string,
) (bytes io.ReadCloser, err error) {
	groups, file := findGroups(bucketName, directoryName, fileName)
	if groups == nil {
		return nil, errors.New("the requested file does not exist")
	}
	var client *clientData
	for _, client = range clients {
		if _, found := client.Buckets[bucketName]; found {
			break
		}
	}
	if client == nil {
		return nil, errors.New("the requested file does not exist")
	}
	return client.Client.Download(bucketName, groups[groupName][file])
}

func findGroups(
	bucketName string,
	directoryName string,
	fileName string,
) (map[string][]*FileInfo, int) {
	bucket := FindBucket(bucketName)
	if bucket == nil {
		return nil, 0
	}

	var dirI int
	for dirI = 0; dirI < len(bucket.Definition); dirI++ {
		if bucket.Definition[dirI].Alias == directoryName {
			break
		}
	}
	if dirI >= len(bucket.Definition) {
		return nil, 0
	}
	dir := bucket.Definition[dirI]
	var fileI int
	for fileI = 0; fileI < len(dir.Files); fileI++ {
		if dir.Files[fileI].Alias == fileName {
			return bucket.groups[dirI], fileI
		}
	}
	return nil, 0
}

func FindBucket(bucketName string) *BucketData {
	for _, client := range clients {
		if bucket, found := client.Buckets[bucketName]; found {
			return bucket
		}
	}
	return nil
}

func FindDirectory(
	bucketName string,
	directoryName string,
) *backup.Directory {
	bucket := FindBucket(bucketName)
	if bucket == nil {
		return nil
	}
	for _, dir := range bucket.Definition {
		if dir.Alias == directoryName {
			return dir
		}
	}
	return nil
}

func FindFile(
	bucketName string,
	directoryName string,
	fileName string,
) *backup.File {
	dir := FindDirectory(bucketName, directoryName)
	if dir == nil {
		return nil
	}
	for _, file := range dir.Files {
		if file.Alias == fileName {
			return file
		}
	}
	return nil
}
