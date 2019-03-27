package jstrace

import (
	"github.com/teamnsrg/mida/log"
	"strconv"
)

func OpenWPMCheckTraceForFingerprinting(trace *JSTrace) error {
	for _, isolate := range trace.Isolates {
		for _, script := range isolate.Scripts {
			fp, err := OpenWPMCheckScript(script)
			if err != nil {
				log.Log.Error(err)
			}
			for k, v := range fp {
				if v {
					log.Log.Warnf("Found Fingerprinting [ %s ]: %s", k, script.BaseUrl)
				}
			}
		}
	}

	return nil
}

func OpenWPMCheckScript(s *Script) (map[string]bool, error) {

	fingerprinting := make(map[string]bool)

	// Canvas fingerprinting state
	imageExtracted := false
	moreThan10Characters := false
	styles := make(map[string]bool)

	//Canvas font fingerprinting state
	fontCalls := 0
	measureTextCalls := 0

	// WebRTC Fingerprinting state
	createDataChannel := false
	createOffer := false
	onIceCandidate := false

	// Audio state
	audioCalls := make(map[string]bool)

	// Battery state
	charging := false
	discharging := false
	level := false

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
						fingerprinting["CANVAS"] = false
					}
				}
			}

			// The script should not call the save, restore, or addEventListener methods of the rendering context
			if call.C == "CanvasRenderingContext2D" && (call.F == "save" || call.F == "restore") {
				fingerprinting["CANVAS"] = false
			}
			if call.C == "HTMLCanvasElement" && call.F == "addEventListener" {
				fingerprinting["CANVAS"] = false
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

			// Canvas font fingerprinting checks
			if call.T == "set" && call.C == "CanvasRenderingContext2D" && call.F == "font" {
				fontCalls += 1
			} else if call.C == "CanvasRenderingContext2D" && call.F == "measureText" {
				measureTextCalls += 1
			}

			//WebRTC Checks
			if call.C == "RTCPeerConnection" {
				if call.F == "createDataChannel" {
					createDataChannel = true
				} else if call.F == "createOffer" {
					createOffer = true
				} else if call.F == "onicecandidate" && call.T == "set" {
					onIceCandidate = true
				}
			}

			// Audio Checks
			if call.C == "BaseAudioContext" {
				audioCalls[call.F] = true
			} else if call.C == "OfflineAudioContext" {
				audioCalls[call.F] = true
			} else if call.C == "ScriptProcessorNode" {
				audioCalls[call.F] = true
			}

			// Battery checks
			if call.C == "BatteryManager" {
				if call.F == "level" || call.F == "onlevelchange" {
					level = true
				} else if call.F == "charging" || call.F == "chargingTime" || call.F == "onchargingchange" || call.F == "onchargingtimechance" {
					charging = true
				} else if call.F == "dischargingTime" || call.F == "ondischargingtimechange" {
					discharging = true
				}
			} else if call.C == "EventTarget" && call.F == "addEventListener" {
				if len(call.Args) > 1 {
					continue
				}

				arg := call.Args[0].Val

				if arg == "levelchange" {
					level = true
				} else if arg == "chargingchange" || arg == "chargingtimechange" {
					charging = true
				} else if arg == "dischargingchange" || arg == "dischargingtimechange" {
					discharging = true
				}
			}
		}
	}

	// Canvas fingerprinting
	if imageExtracted && (moreThan10Characters || len(styles) >= 2) {
		if _, ok := fingerprinting["CANVAS"]; !ok {
			fingerprinting["CANVAS"] = true
		}
		// Otherwise, just leave it as false based on some criterion we found to exclude
	} else {
		fingerprinting["CANVAS"] = false
	}

	// Canvas font fingerprinting
	if measureTextCalls >= 50 && fontCalls >= 50 {
		fingerprinting["CANVASFONT"] = true
	} else {
		fingerprinting["CANVASFONT"] = false
	}

	// WebRTC fingerprinting
	if createOffer && createDataChannel && onIceCandidate {
		fingerprinting["WEBRTC"] = true
	} else {
		fingerprinting["WEBRTC"] = false
	}

	// Audio fingerprinting
	fingerprinting["AUDIO"] = false
	if _, ok := audioCalls["createOscillator"]; ok {
		if _, ok = audioCalls["createDynamicsCompressor"]; ok {
			if _, ok = audioCalls["startRendering"]; ok {
				if _, ok = audioCalls["oncomplete"]; ok {
					fingerprinting["AUDIO"] = true
				}
			}
		} else if _, ok = audioCalls["createAnalyser"]; ok {
			_, scriptProcessor := audioCalls["scriptProcessor"]
			_, createGain := audioCalls["createGain"]
			_, destination := audioCalls["destination"]
			_, onaudioprocess := audioCalls["onaudioprocess"]

			if scriptProcessor && createGain && destination && onaudioprocess {
				fingerprinting["AUDIO"] = true
			}
		}
	}

	// Battery fingerprinting
	if charging && discharging && level {
		fingerprinting["BATTERY"] = true
	} else {
		fingerprinting["BATTERY"] = false
	}

	return fingerprinting, nil
}
