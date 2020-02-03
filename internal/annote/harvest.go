package annote

import (
	"log"
	"time"
)

//
// Harvester
//

var (
	harvestControl chan int
	// should have a mutex protecting it
	harvestStatus int
)

const (
	HNow = iota
	HExit

	StatusWaiting = iota
	StatusHarvesting
)

func BackgroundHarvester() {
	var lastHarvest time.Time
	var harvestInterval time.Duration
	// we store our config as a special "curate item"

	config, err := Datasource.FindItem("system")
	if err == nil {
		s := config.FirstField("last-harvest")
		if s != "" {
			lastHarvest, _ = time.Parse(time.RFC3339, s)
		}
		s = config.FirstField("harvest-interval")
		if s != "" {
			harvestInterval, _ = time.ParseDuration(s)
		}
	}

	harvestControl = make(chan int, 100)

	for {
		harvestStatus = StatusWaiting
		var timer <-chan time.Time
		if harvestInterval > 0 {
			timer = time.After(harvestInterval)
		}
		select {
		case msg := <-harvestControl:
			if msg == HExit {
				return
			}
		case <-timer:
		}
		log.Println("Start Harvest since", lastHarvest)
		harvestStatus = StatusHarvesting
		t := time.Now()
		c := make(chan CurateItem, 10)
		go func() {
			for item := range c {
				err := Datasource.IndexItem(item)
				if err != nil {
					log.Println(err)
				}
				SearchEngine.IndexRecord(item)
			}
		}()
		err := HarvestCurateObjects(TargetFedora, lastHarvest, func(item CurateItem) error {
			c <- item
			return nil
		})

		if err != nil {
			log.Println(err)
		} else {
			lastHarvest = t
			config, err := Datasource.FindItem("system")
			if err == nil {
				config.RemoveAll("last-harvest")
				config.Add("last-harvest", t.Format(time.RFC3339))
				Datasource.IndexItem(config)
			}
		}
		log.Println("Finish Harvest")
	}
}
