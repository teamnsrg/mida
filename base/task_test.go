package base

import (
	"testing"
)

// TestReadWriteReadTask reads the simplest possible task from bytes, writes it back
// out to bytes, and reads it again, to ensure that the read and write task process
// is compatible.
func TestReadWriteReadTask(t *testing.T) {
	t.Parallel()

	tasks, err := ReadTasksFromBytes([]byte(`{"url": "illinois.edu"}`))
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatal("failed to read task correctly")
	}

	taskBytes, err := WriteTaskSliceToBytes(tasks)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err = ReadTasksFromBytes(taskBytes)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 || *tasks[0].URL != "illinois.edu" {
		t.Fatal("failed to read written task bytes into valid task slice")
	}
}
