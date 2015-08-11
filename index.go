package gnosis

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/JackKnifed/blackfriday"
	"github.com/blevesearch/bleve"
	"github.com/JackKnifed/blackfriday-text"
	"gopkg.in/fsnotify.v1"
)

var openWatchers []fsnotify.Watcher

type indexedPage struct {
	Title     string    `json:"title"`
	URIPath string    `json:"path"`
	Body     string    `json:"body"`
	Topics   string    `json:"topic"`
	Keywords string    `json:"keyword"`
	Authors  string    `json: "author"`
	Modified time.Time `json:"modified"`
}

func createIndex(config IndexSection) bool {
	newIndex, err := bleve.Open(path.Clean(config.IndexPath))
	if err == nil {
		log.Printf("Index already exists %s", config.IndexPath)
	} else if err == bleve.ErrorIndexPathDoesNotExist {
		log.Printf("Creating new index %s", config.IndexName)
		// create a mapping
		indexMapping := buildIndexMapping(config)
		newIndex, err = bleve.New(path.Clean(config.IndexPath), indexMapping)
		if err != nil {
			log.Printf("Failed to create the new index %s - %v", config.IndexPath, err)
			return false
		} else {
		}
	} else {
		log.Printf("Got an error opening the index %s but it already exists %v", config.IndexPath, err)
		return false
	}
	newIndex.Close()
	return true
}

func EnableIndex(config IndexSection) bool {
	if ! createIndex(config) {
		return false
	}
	for dir, path := range config.WatchDirs {
		// dir = strings.TrimSuffix(dir, "/")
		log.Printf("Watching and walking dir %s index %s", dir, config.IndexPath)
		walkForIndexing(dir, dir, path, config)
	}
	return true
}

func DisableAllIndexes() {
	log.Println("Stopping all watchers")
	for _, watcher := range openWatchers {
		watcher.Close()
	}
}

func buildIndexMapping(config IndexSection) *bleve.IndexMapping {

	// create a text field type
	enTextFieldMapping := bleve.NewTextFieldMapping()
	enTextFieldMapping.Analyzer = config.IndexType

	// create a date field type
	dateTimeMapping := bleve.NewDateTimeFieldMapping()

	// map out the wiki page
	wikiMapping := bleve.NewDocumentMapping()
	wikiMapping.AddFieldMappingsAt("title", enTextFieldMapping)
	wikiMapping.AddFieldMappingsAt("path", enTextFieldMapping)
	wikiMapping.AddFieldMappingsAt("body", enTextFieldMapping)
	wikiMapping.AddFieldMappingsAt("topic", enTextFieldMapping)
	wikiMapping.AddFieldMappingsAt("keyword", enTextFieldMapping)
	wikiMapping.AddFieldMappingsAt("author", enTextFieldMapping)
	wikiMapping.AddFieldMappingsAt("modified", dateTimeMapping)

	// add the wiki page mapping to a new index
	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping(config.IndexName, wikiMapping)
	indexMapping.DefaultAnalyzer = config.IndexType

	return indexMapping
}

// walks a given path, and runs processUpdate on each File
func walkForIndexing(path, filePath, requestPath string, config IndexSection) {
	watcherLoop(path, filePath, requestPath, config)
	dirEntries, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, dirEntry := range dirEntries {
		dirEntryPath := path + string(os.PathSeparator) + dirEntry.Name()
		if dirEntry.IsDir() {
			walkForIndexing(dirEntryPath, filePath, requestPath, config)
		} else if strings.HasSuffix(dirEntry.Name(), config.WatchExtension) {
			processUpdate(dirEntryPath, getURIPath(dirEntryPath, filePath, requestPath), config)
		}
	}
}

// given all of the inputs, watches afor new/deleted files in that directory
// adds/removes/udpates index as necessary
func watcherLoop(watchPath, filePrefix, uriPrefix string, config IndexSection) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	err = watcher.Add(watchPath)
	if err != nil {
		log.Fatal(err)
	}

	idleTimer := time.NewTimer(10 * time.Second)
	queuedEvents := make([]fsnotify.Event, 0)

	openWatchers = append(openWatchers, *watcher)

	log.Printf("watching '%s' for changes...", watchPath)

	for {
		select {
		case event := <-watcher.Events:
			queuedEvents = append(queuedEvents, event)
			idleTimer.Reset(10 * time.Second)
		case err := <-watcher.Errors:
			log.Fatal(err)
		case <-idleTimer.C:
			for _, event := range queuedEvents {
				if strings.HasSuffix(event.Name, config.WatchExtension) {
					switch event.Op {
					case fsnotify.Remove, fsnotify.Rename:
						// delete the filePath
						processDelete(getURIPath(watchPath + event.Name, filePrefix, uriPrefix),
							config.IndexName)
					case fsnotify.Create, fsnotify.Write:
						// update the filePath
						processUpdate(watchPath + event.Name,
							getURIPath(watchPath + event.Name, filePrefix, uriPrefix), config)
					default:
						// ignore
					}
				}
			}
			queuedEvents = make([]fsnotify.Event, 0)
			idleTimer.Reset(10 * time.Second)
		}
	}
}

// Update the entry in the index to the output from a given file
func processUpdate(filePath, uriPath string, config IndexSection) {
	page, err := generateWikiFromFile(filePath, uriPath, config.Restricted)
	if err != nil {
		log.Print(err)
	} else {
		index, _ := bleve.Open(config.IndexPath)
		defer index.Close()
		index.Index(uriPath, page)
		log.Printf("updated: %s as %s", filePath, uriPath)
	}
}

// Deletes a given path from the wiki entry
func processDelete(uriPath, indexPath string) {
	log.Printf("delete: %s", uriPath)
	index, _ := bleve.Open(indexPath)
	defer index.Close()
	err := index.Delete(uriPath)
	if err != nil {
		log.Print(err)
	}
}

func cleanupMarkdown(input []byte) string {
	extensions := 0 | blackfriday.EXTENSION_ALERT_BOXES
	renderer := blackfridaytext.TextRenderer()
	output := blackfriday.Markdown(input, renderer, extensions)
	return string(output)
}

func generateWikiFromFile(filePath, uriPath string, restrictedTopics []string) (*indexedPage, error) {
	pdata := new(PageMetadata)
	err := pdata.LoadPage(filePath)
	if err != nil {
		return nil, err
	}

	if pdata.MatchedTopic(restrictedTopics) == true {
		return nil, errors.New("Hit a restricted page - " + pdata.Title)
	} 

	topics, keywords, authors := pdata.ListMeta()
	rv := indexedPage{
		Title:     pdata.Title,
		Body:     cleanupMarkdown(pdata.Page),
		URIPath: uriPath,
		Topics:   strings.Join(topics, " "),
		Keywords: strings.Join(keywords, " "),
		Authors: strings.Join(authors, " "),
		Modified: pdata.FileStats.ModTime(),
	}

	return &rv, nil
}

func getURIPath(filePath, filePrefix, uriPrefix string) (uriPath string) {
	uriPath = strings.TrimPrefix(filePath, filePrefix)
	uriPath = uriPrefix + uriPath
	return
}

