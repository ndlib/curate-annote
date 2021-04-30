package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/ndlib/curate-annote/internal/annote"
)

type Config struct {
	Mysql                 string
	Fedora                string
	StaticFilePath        string
	TemplatePath          string
	Port                  string
	CurateURL             string
	AnnotationStore       string
	AnnotationCredentials string
	ImageViewerHost       string
	Hostname              string
	SolrHost              string
	UploadPath            string
	ElasticSearchURL      string
}

var (
	fedora *annote.RemoteFedora
	db     *annote.MysqlDB
)

func main() {
	log.Println("==========")
	config := Config{
		Mysql:          "",
		Fedora:         os.Getenv("FEDORA_PATH"),
		Port:           "8080",
		TemplatePath:   "./web/templates",
		StaticFilePath: "./web/static",
	}
	configFile := flag.String("config-file", "", "Configuration File to use")
	flag.Parse()
	if *configFile != "" {
		log.Println("Using config file", *configFile)
		if _, err := toml.DecodeFile(*configFile, &config); err != nil {
			log.Println(err)
			return
		}
	}

	if config.Fedora == "" {
		log.Println("FEDORA_PATH not set")
		return
	}
	fedora = annote.NewRemote(config.Fedora)
	annote.TargetFedora = fedora
	if config.Mysql != "" {
		var err error
		db, err = annote.NewMySQL(config.Mysql)
		if err != nil {
			log.Println(err)
			return
		}
		annote.Datasource = db
	}
	annote.CurateURL = config.CurateURL
	annote.FileStore.Root = config.UploadPath

	if config.AnnotationStore != "" {
		annote.AnnotationStore = &annote.AnnoStore{
			Host:             config.AnnotationStore,
			UsernamePassword: config.AnnotationCredentials,
			ImageViewerHost:  config.ImageViewerHost,
			OurURL:           config.Hostname,
		}
	}

	// Prefer ES over Solr as search engine since our solr implementation
	// doesn't support indexing new records.
	if config.ElasticSearchURL != "" {
		log.Println("Using ElasticSearch", config.ElasticSearchURL)
		annote.SearchEngine = annote.NewElasticSearch(config.ElasticSearchURL)
	} else if config.SolrHost != "" {
		log.Println("Using Solr", config.SolrHost)
		annote.SearchEngine = &annote.SolrInfo{Host: config.SolrHost}
	} else {
		log.Println("Using Database for search engine")
		annote.SearchEngine = annote.Datasource
	}

	if config.TemplatePath != "" {
		err := annote.LoadTemplates(config.TemplatePath)
		if err != nil {
			log.Println(err)
		}
		annote.StaticFilePath = config.StaticFilePath
		// add routes
		h := annote.AddRoutes()
		log.Println("Starting Background harvester")
		go annote.BackgroundHarvester()
		log.Println("Starting Background annotation store worker")
		annote.StartBackgroundProcess()
		err = http.ListenAndServe(":"+config.Port, h)
		log.Println("ListenAndServe:", err)
	}
}
