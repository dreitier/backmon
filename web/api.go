package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/dreitier/backmon/backup"
	"github.com/dreitier/backmon/config"
	"github.com/dreitier/backmon/storage"
)

func GetDisks(w http.ResponseWriter) {
	disks := storage.GetDisks()
	names := make([]string, 0, len(disks))

	for _, disk := range disks {
		names = append(names, disk.Name)
	}

	sort.Strings(names)

	writeData(w, rootResource{
		Links:    links{relSelf: linkTo()},
		Embedded: embedded{relDisks: embed(names)},
	})
}

func GetDirectories(
	w http.ResponseWriter,
	diskName string,
) {
	definition := findDefinition(w, diskName)
	if definition == nil {
		return
	}

	names := make([]string, 0, len(definition.Directories))

	for _, dir := range definition.Directories {
		names = append(names, dir.Alias)
	}

	disk := diskResource{
		Name:  diskName,
		Quota: definition.Quota,
		Links: links{
			relSelf: linkTo(diskName),
			relUp:   linkTo(),
		},
		Embedded: embedded{relDirectories: embed(names, diskName)},
	}

	if definition.Location != nil {
		disk.Timezone = definition.Location.String()
	}

	writeData(w, disk)
}

func GetFiles(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
) {
	dir := findDirectory(w, diskName, directoryName)
	if dir == nil {
		return
	}

	names := make([]string, 0, len(dir.Files))

	for _, file := range dir.Files {
		names = append(names, file.Alias)
	}

	writeData(w, namedResource{
		Name: directoryName,
		Links: links{
			relSelf: linkTo(diskName, directoryName),
			relUp:   linkTo(diskName),
		},
		Embedded: embedded{relFiles: embed(names, diskName, directoryName)},
	})
}

func GetVariations(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
	fileName string,
) {
	filenames := storage.GetFilenames(diskName, directoryName, fileName)
	if filenames == nil {
		fileNotFound(w, fileName)
		return
	}

	sort.Strings(filenames)

	variants := make([]embeddedResource, 0, len(filenames))

	for _, variant := range filenames {
		resource := embeddedResource{Name: variant}

		// a variant is only reachable if downloads are enabled, as there is no route for it otherwise
		if config.GetInstance().Downloads().Enabled {
			resource.Links = links{relDownload: linkTo(diskName, directoryName, fileName, variant)}
		}

		variants = append(variants, resource)
	}

	writeData(w, namedResource{
		Name: fileName,
		Links: links{
			relSelf: linkTo(diskName, directoryName, fileName),
			relUp:   linkTo(diskName, directoryName),
		},
		Embedded: embedded{relVariants: variants},
	})
}

func Download(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
	fileName string,
	variation string,
) {
	data, length, contentType, err := storage.Download(diskName, directoryName, fileName, variation)
	if err != nil {
		log.Debugf("Download of %s/%s/%s/%s failed: %s", diskName, directoryName, fileName, variation, err)
		groupNotFound(w, variation)
		return
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")

	_, err = io.Copy(w, data)
	_ = data.Close()

	if err != nil {
		// headers and parts of the body have already been sent, so the status code can't be changed anymore
		log.Errorf("Failed to stream file %s to client: %v", fileName, err)
	}
}

func writeData(w http.ResponseWriter, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errStr := err.Error()
		_, _ = w.Write([]byte(errStr))
		return
	}
	w.Header().Set("Content-Type", halContentType)
	_, err = w.Write(b)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// findDefinition returns the backup definitions of a disk or writes a 404 response
func findDefinition(
	w http.ResponseWriter,
	diskName string,
) *backup.Definition {
	definition, found := storage.GetDefinition(diskName)
	if !found {
		diskNotFound(w, diskName)
		return nil
	}
	if definition == nil {
		definitionsNotFound(w, diskName)
	}
	return definition
}

func findDirectory(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
) *backup.Directory {
	definition := findDefinition(w, diskName)
	if definition == nil {
		return nil
	}
	for _, dir := range definition.Directories {
		if dir.Alias == directoryName {
			return dir
		}
	}
	directoryNotFound(w, directoryName)
	return nil
}
