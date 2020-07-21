package fetch

import (
	b "github.com/teamnsrg/mida/base"
	"math/rand"
	"time"
)

func FromFile(fileName string, shuffle bool) (<-chan *b.RawTask, error) {
	taskSet, err := b.ReadTasksFromFile(fileName)
	if err != nil {
		return nil, err
	}

	if shuffle {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(taskSet),
			func(i, j int) { taskSet[i], taskSet[j] = taskSet[j], taskSet[i] })
	}

	res := make(chan *b.RawTask)

	go func() {
		for _, task := range taskSet {
			taskCopy := task
			res <- &taskCopy
		}
		close(res)
	}()

	return res, nil
}
