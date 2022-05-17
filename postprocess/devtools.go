package postprocess

import (
	"bufio"
	"errors"
	"github.com/chromedp/cdproto/debugger"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	pp "github.com/teamnsrg/profparse"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var covMapping map[string]int
var covMappingLength int
var covMappingLock sync.Mutex

func DevTools(rr *b.RawResult) (b.FinalResult, error) {
	finalResult := b.FinalResult{
		Summary:            rr.TaskSummary,
		DTResourceMetadata: make(map[string]b.DTResource),
		DTScriptMetadata:   make(map[string]*debugger.EventScriptParsed),
	}

	finalResult.Summary.TaskTiming.BeginPostprocess = time.Now()

	// For brevity
	tw := finalResult.Summary.TaskWrapper
	st := tw.SanitizedTask
	log.Log.WithField("URL", st.URL).Debug("Begin Postprocess")

	// Ignore any requests/responses which do not have a matching request/response
	if *st.DS.ResourceMetadata {
		for k := range rr.DevTools.Network.RequestWillBeSent {
			if _, ok := rr.DevTools.Network.ResponseReceived[k]; ok {

				/*
					var tdl int64 = -1
					if _, okData := rr.DataLengths[k]; okData {
						tdl = rawResult.DataLengths[k]
					}
				*/

				finalResult.DTResourceMetadata[k] = b.DTResource{
					Requests: rr.DevTools.Network.RequestWillBeSent[k],
					Response: rr.DevTools.Network.ResponseReceived[k],
					// TotalDataLength: tdl,
				}

			}
		}
	}

	if *st.DS.ScriptMetadata {
		for _, v := range rr.DevTools.Scripts {
			if _, ok := finalResult.DTScriptMetadata[v.ScriptID.String()]; ok {
				rr.TaskSummary.TaskWrapper.Log.Warnf("found duplicate scriptId: %s", v.ScriptID.String())
			} else {
				finalResult.DTScriptMetadata[v.ScriptID.String()] = v
			}
		}
	}

	if *st.DS.Cookies {
		finalResult.DTCookies = rr.DevTools.Cookies
	}

	if *st.DS.DOM {
		finalResult.DTDOM = rr.DevTools.DOM
	}

	finalResult.Summary.NavURL = st.URL
	finalResult.Summary.UUID = finalResult.Summary.TaskWrapper.UUID.String()
	finalResult.Summary.NumResources = len(rr.DevTools.Network.RequestWillBeSent)

	if *st.OPS.SftpOut.Enable {
		finalResult.Summary.OutputHost = *st.OPS.SftpOut.Host
		finalResult.Summary.OutputPath = *st.OPS.SftpOut.Path
	}

	if *st.DS.BrowserCoverage {
		covPath := path.Join(tw.TempDir, b.DefaultCoverageSubdir)
		files, err := ioutil.ReadDir(covPath)
		var rawCovFilenames []string
		if err != nil {
			log.Log.Error("no coverage directory was present")
		} else {
			for _, f := range files {
				if strings.HasSuffix(f.Name(), "profraw") {
					rawCovFilenames = append(rawCovFilenames, path.Join(covPath, f.Name()))
				}
			}
		}

		finalResult.Summary.RawCoverageFilenames = rawCovFilenames

		if len(rawCovFilenames) > 0 {
			err = pp.MergeProfraws(rawCovFilenames, path.Join(covPath, "coverage.profdata"), "/usr/bin/llvm-profdata", 1)
			if err != nil {
				log.Log.Error(err)
			} else {
				err = pp.GenCustomCovTxtFileFromProfdata(path.Join(covPath, "coverage.profdata"), "/usr/bin/chrome_unstripped",
					path.Join(covPath, "coverage.txt"), "/usr/bin/llvm-cov-custom", 1)
				if err != nil {
					log.Log.Error(err)
					bytes, err := ioutil.ReadFile(path.Join(covPath, "coverage.txt"))
					if err != nil {
						log.Log.Error(err)
					}
					log.Log.Info(string(bytes))
				} else {
					covMap, err := pp.ReadFileToCovMap(path.Join(covPath, "coverage.txt"))
					if err != nil {
						log.Log.Error(err)
					} else {
						bv := pp.ConvertCovMapToBools(covMap)
						err = pp.WriteFileFromBV(path.Join(covPath, b.DefaultCovBVFileName), bv)
						if err != nil {
							log.Log.Error(err)
						}
					}
				}
			}
		}

		// Clean up profraw and text files
		files, err = ioutil.ReadDir(covPath)
		for _, file := range files {
			if strings.HasSuffix(file.Name(), "profraw") ||
				strings.HasSuffix(file.Name(), "txt") ||
				strings.HasSuffix(file.Name(), "profdata") {
				err = os.Remove(path.Join(covPath, file.Name()))
				if err != nil {
					log.Log.Error(err)
				}
			}
		}
	}

	log.Log.WithField("URL", st.URL).Debug("End Postprocess")
	finalResult.Summary.TaskTiming.EndPostprocess = time.Now()

	return finalResult, nil
}

func ParseMergedTextfile(fname string, mapping map[string]int) ([]bool, int, error) {
	if covMappingLength == 0 || covMapping == nil {
		return nil, 0, errors.New("coverage map has not been initialized")
	}

	result := make([]bool, covMappingLength)

	f, err := os.Open(fname)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	fn := ""
	index := -1
	counters := false
	coveredBlocks := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			fn = ""
			index = -1
			counters = false
			continue
		}

		if fn == "" {
			fn = strings.TrimSpace(line)
			if _, ok := mapping[fn]; !ok {
				return nil, 0, errors.New("unknown function: " + fn)
			}
			index = mapping[fn]
			continue
		}

		if strings.HasPrefix(line, "# Counter V") {
			counters = true
			continue
		}

		if counters {
			executions, err := strconv.Atoi(line)
			if err != nil {
				return nil, 0, err
			}
			if executions > 0 {
				result[index] = true
				coveredBlocks += 1
			}
			index++
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, 0, err
	}

	return result, coveredBlocks, nil
}

func boolsToBytes(t []bool) []byte {
	bb := make([]byte, (len(t)+7)/8)
	for i, x := range t {
		if x {
			bb[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return bb
}

func bytesToBools(bb []byte) []bool {
	t := make([]bool, 8*len(bb))
	for i, x := range bb {
		for j := 0; j < 8; j++ {
			if (x<<uint(j))&0x80 == 0x80 {
				t[8*i+j] = true
			}
		}
	}
	return t[:len(t)-(8-(covMappingLength%8))]
}

func WriteCovFile(fName string, bv []bool) error {
	f, err := os.Create(fName)
	if err != nil {
		return err
	}

	_, err = f.Write(boolsToBytes(bv))
	if err != nil {
		return err
	}

	return nil
}
