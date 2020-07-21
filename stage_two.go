package main

import (
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/sanitize"
	"sync"
)

// stage2 takes raw tasks from stage1 and produces sanitized tasks for
func stage2(rawTaskChan <-chan *b.RawTask, sanitizedTaskChan chan<- *b.TaskWrapper, pipelineWG *sync.WaitGroup) {
	for r := range rawTaskChan {
		st, err := sanitize.Task(r)
		if err != nil {
			log.Log.Error(err)
			continue
		}
		pipelineWG.Add(1)

		sanitizedTaskChan <- &st
	}

	// Wait until the pipeline is clear before we close the sanitized task channel,
	// which will cause MIDA to shutdown
	pipelineWG.Wait()
	close(sanitizedTaskChan)

	return
}
