package fs

// Common data structures for files. As S3 objects are also files, we are using our own filesystem abstraction.
import (
	"fmt"
	fs "github.com/dreitier/backmon/storage/fs"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

const DotStatFileSuffix = ".stat"

// DotStatYaml is a simple YAML file, containing statistics about a file
type DotStatYaml struct {
	BornAt     *string `yaml:"born_at,omitempty"`
	ModifiedAt *string `yaml:"modified_at,omitempty"`
	ArchivedAt *string `yaml:"archived_at,omitempty"`
}

func ApplyDotStatValuesRecursively(dotStatFileSources map[string] /* absolute path of file */ string /*absolute path to .stat file*/, directoryInfo *fs.DirectoryInfo) {
	ApplyDotStatValues(dotStatFileSources, directoryInfo.Files)

	for _, subdirectory := range directoryInfo.SubDirs {
		ApplyDotStatValuesRecursively(dotStatFileSources, subdirectory)
	}
}

// ApplyDotStatValues For the provided map, each .stat file for an existing backup is parsed and then applied to the backup's stat (born_at, modified_at, archived_at) attributes
func ApplyDotStatValues(dotStatFileSources map[string] /* absolute path of file */ string /*absolute path to .stat file*/, files []*fs.FileInfo) {
	for _, fileInfo := range files {
		absolutePathToNonStatFile := fileInfo.Parent + "/" + fileInfo.Name

		if pathToStatFile, ok := dotStatFileSources[absolutePathToNonStatFile]; ok {
			log.Debugf("%s: applying .stat file at %s", absolutePathToNonStatFile, pathToStatFile)

			_, err := updateStatAttributesFromYamlValues(fileInfo, pathToStatFile)

			if err != nil {
				log.Warnf("Could not parse stat file %s: %s", pathToStatFile, err)
				continue
			}

			log.Debugf("%s: stat file %s has been applied", fileInfo.Name, pathToStatFile)
		}
	}
}

// ToDotStatPath Appends the `.stat` suffix to the provide file path
func ToDotStatPath(pathToOriginalFile string) string {
	return pathToOriginalFile + DotStatFileSuffix
}

// IsStatFile Return true if the file name or path has a `.stat` suffix
func IsStatFile(fileName string) bool {
	return strings.HasSuffix(fileName, DotStatFileSuffix)
}

// RemoveDotStatSuffix Removes the `.stat` suffix from the provided file path if present
func RemoveDotStatSuffix(pathToDotStatFile string) string {
	if strings.HasSuffix(pathToDotStatFile, DotStatFileSuffix) {
		pathToDotStatFile = pathToDotStatFile[:len(pathToDotStatFile)-len(DotStatFileSuffix)]
	}

	return pathToDotStatFile
}

// From the provided YAML file the keys are read an then accordingly applied to the file's stat attributes (BornAt, ModifiedAt, ArchivedAt)
func updateStatAttributesFromYamlValues(fileInfo *fs.FileInfo, pathToStatFile string) (*DotStatYaml, error) {
	// TODO: fix deprecation
	buf, err := ioutil.ReadFile(pathToStatFile)
	if err != nil {
		return nil, err
	}

	c := &DotStatYaml{}
	err = yaml.Unmarshal(buf, c)

	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal: %v", err)
	}

	updateTimeField(c.BornAt, &fileInfo.BornAt)
	updateTimeField(c.ModifiedAt, &fileInfo.ModifiedAt)
	updateTimeField(c.ArchivedAt, &fileInfo.ArchivedAt)

	return c, nil
}

func updateTimeField(content *string, targetTime *time.Time) {
	if content == nil {
		return
	}

	i, err := strconv.ParseInt(*content, 10, 64)

	if err != nil {
		log.Debugf("Unable to parse '%s': %s", *content, err)
		// ignore any parsing errors
		return
	}

	*targetTime = time.Unix(i, 0)
}
