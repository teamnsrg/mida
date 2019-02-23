package main

import "testing"

func TestReadTask(t *testing.T) {
	tasks, err := ReadTasksFromFile("examples/exampleTask.json")
	if err != nil {
		t.Error("Failed to read exampleTask.json: ", err)
	}

	if len(tasks) != 3 {
		t.Fatal("Wrong length for tasks read from file")
	}

	if tasks[1].Output.Path != "example_results" {
		t.Fatal("Incorrect output path setting for task")
	}
}

