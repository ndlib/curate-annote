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
	s, err := Datasource.ReadConfig("last-harvest")
	if err == nil {
		lastHarvest, _ = time.Parse(time.RFC3339, s)
	}
	s, err = Datasource.ReadConfig("harvest-interval")
	if err == nil {
		harvestInterval, _ = time.ParseDuration(s)
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
			Datasource.SetConfig("last-harvest", t.Format(time.RFC3339))
		}
		log.Println("Finish Harvest")
	}
}
