package annote

import (
	"log"
	"time"
)

type rapWorkerResult struct {
	NumNotComplete int
	Err            error
}

// backgroundRAPchecker will poll the annotation server to track the statuses
// of RAP ingests. It adjusts its polling interval depending on whether there
// are any pending jobs. Send something on the channel c to notify the poller
// that there is a new pending job to monitor.
func backgroundRAPchecker(c <-chan struct{}) {
	// We want to balance being timely with updates without polling the
	// annotation server too much. This is accomplished in two ways:
	// First, we only poll when there is an item that is not complete.
	// Second, we set a limit that we will not poll the annotation server
	// more frequently than.
	const minInterval = 30 * time.Second
	workerResult := make(chan rapWorkerResult)
	nextpoll := time.Now().Add(minInterval) // so even after restarts, we always wait the time period
	inDeepSleep := false
	for {
		d := minInterval
		if inDeepSleep {
			d = 24 * time.Hour
		}
	sleeploop:
		log.Println("@@@ sleeping", d)
		select {
		case <-c:
			// there is something we should monitor, so shorten our timeout
			inDeepSleep = false
			log.Println("@@@ receive signal")

		case <-time.After(d):
			log.Println("@@@ receive timer")
		}

		// if polling too soon, go back to sleep
		if time.Now().Before(nextpoll) {
			d = nextpoll.Sub(time.Now())
			goto sleeploop // so we can supply custom duration
		}

		// do a single poll....
		nextpoll = time.Now().Add(minInterval)
		go checkRAPs(workerResult)
		inDeepSleep = true
	keep_waiting:
		// wait for response
		log.Println("@@@ waiting for status")
		select {
		case <-c: // eat these so nothing else blocks
			log.Println("@@@@ receive signal")
			inDeepSleep = false
			goto keep_waiting
		case r := <-workerResult:
			if r.Err != nil || r.NumNotComplete > 0 {
				// need to do another poll
				inDeepSleep = false
			}
		}
	}
}

func checkRAPs(out chan<- rapWorkerResult) {
	log.Println("polling Annotation Store")
	raps, err := AnnotationStore.RAPStatus()
	if err != nil {
		log.Println(err)
		out <- rapWorkerResult{Err: err}
		return
	}
	UUIDs, err := Datasource.SearchItemUUID("", "", "")
	if err != nil {
		log.Println(err)
		out <- rapWorkerResult{Err: err}
		return
	}

	notComplete := 0
	var bigerr error
big_loop:
	for _, record := range UUIDs {
		if record.Status == "a" {
			// complete. skip it for now
			continue
		}
		// is there an entry for this UUID in that status update?
		log.Println("searching for", record)
		for i := range raps {
			if raps[i].UUID != record.UUID {
				continue
			}

			rap := raps[i]
			log.Println("found", rap)
			// also verify the ID...?
			if rap.Status.Code != "a" {
				notComplete++
			}
			if rap.Status.Code == record.Status {
				break // nothing to update
			}
			// update our record to the new status
			record.Status = rap.Status.Code
			err = Datasource.UpdateUUID(record)
			if err != nil {
				log.Println(err)
				bigerr = err
				continue big_loop
			}
			break
		}
	}

	out <- rapWorkerResult{NumNotComplete: notComplete, Err: bigerr}
}
