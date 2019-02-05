package main

import "github.com/prometheus/common/log"

type RawMIDAResult struct {
	resultNum int
	stats     TaskStats
}

type FinalMIDAResult struct {
	resultNum int
	stats     TaskStats
}

func ValidateResult(rr chan RawMIDAResult, fr chan FinalMIDAResult) {
	for rawResult := range rr {
		log.Info("validate result here", rawResult)
		finalResult := FinalMIDAResult{}
		fr <- finalResult
	}
}
