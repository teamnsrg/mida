package jstrace

import (
	"github.com/teamnsrg/mida/log"
	"strconv"
)

func OpenWPMCheckTraceForFingerprinting(trace *JSTrace) error {
	for _, isolate := range trace.Isolates {
		for _, script := range isolate.Scripts {
			err := OpenWPMCheckScript(script)
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	return nil
}

func OpenWPMCheckScript(s *Script) error {

	positive, err := OpenWPMCheckCanvasFingerprinting(s)
	if err != nil {
		return err
	}
	if positive {
		log.Log.Warn("Found canvas fingerprinting script: ", s.BaseUrl)
	}

	return nil
}

func OpenWPMCheckCanvasFingerprinting(s *Script) (bool, error) {

	imageExtracted := false
	moreThan10Characters := false
	styles := make(map[string]bool)

	for _, execution := range s.Executions {
		for _, call := range execution.Calls {

			// If we ever set the canvas height or width smaller than 16 pixels, return false
			if call.T == "set" && call.C == "HTMLCanvasElement" && (call.F == "height" || call.F == "width") {
				if len(call.Args) > 0 {
					val, err := strconv.Atoi(call.Args[0].Val)
					if err != nil {
						continue
					}
					if val < 16 {
						return false, nil
					}
				}
			}

			// The script should not call the save, restore, or addEventListener methods of the rendering context
			if call.C == "CanvasRenderingContext2D" && (call.F == "save" || call.F == "restore") {
				return false, nil
			}
			if call.C == "HTMLCanvasElement" && call.F == "addEventListener" {
				return false, nil
			}

			// The script must extract an image with toDataURL or getImageData
			if call.C == "CanvasRenderingContext2D" && call.F == "getImageData" {
				if len(call.Args) >= 4 {
					width, err := strconv.Atoi(call.Args[2].Val)
					if err != nil {
						log.Log.Warn(err)
						continue
					}
					height, err := strconv.Atoi(call.Args[3].Val)
					if err != nil {
						log.Log.Warn(err)
						continue
					}

					if height >= 16 && width >= 16 {
						imageExtracted = true
					}
				}
			}
			if call.C == "HTMLCanvasElement" && call.F == "toDataURL" {
				imageExtracted = true
			}

			// The script must write at least 10 characters or at least 2 colors
			if call.C == "CanvasRenderingContext2D" && (call.F == "fillText" || call.F == "strokeText") {
				if len(call.Args) > 0 {
					text := call.Args[0].Val
					charMap := make(map[rune]bool)
					for _, character := range text {
						charMap[character] = true
					}
					if len(charMap) >= 10 {
						moreThan10Characters = true
					}
				}
			}
			if call.C == "CanvasRenderingContext2D" && (call.F == "fillStyle" || call.F == "strokeStyle") {
				if len(call.Args) > 0 {
					styles[call.Args[0].Val] = true
				}
			}
		}
	}

	if imageExtracted && (moreThan10Characters || len(styles) >= 2) {
		return true, nil
	} else {
		return false, nil
	}
}
