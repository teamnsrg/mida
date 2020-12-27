package postprocess

import (
	"bufio"
	"encoding/csv"
	"errors"
	"github.com/chromedp/cdproto/debugger"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"io/ioutil"
	"os"
	"os/exec"
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

	finalResult.Summary.Url = st.URL
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

		if len(rawCovFilenames) > 0 {
			covMappingLock.Lock()
			if covMapping == nil {
				log.Log.Debug("Building coverage map...")
				buildCovMapping(covPath, rawCovFilenames, "", "")
				log.Log.Debug("coverage map building is complete")
			}
			covMappingLock.Unlock()

			cmd := exec.Command("llvm-profdata", append([]string{"merge",
				"--text", "--failure-mode=any", "--num-threads=1", "--sparse",
				"--output", path.Join(covPath, "merged.txt")}, rawCovFilenames...)...)

			err = cmd.Run()
			if err != nil {
				log.Log.Error("Could not run llvm-profdata successfully. It might not be in your path, or you might have passed" +
					"in corrupted .profraw files.")
			} else {
				bv, coveredBlocks, err := ParseMergedTextfile(path.Join(covPath, "merged.txt"), covMapping)
				if err != nil {
					log.Log.Error(err)
				} else {
					log.Log.Debugf("Covered %d blocks out of %d", coveredBlocks, len(bv))
					err = WriteCovFile(path.Join(covPath, "coverage.bv"), bv)
					if err != nil {
						log.Log.Error(err)
					}
				}
			}

		}

		// Clean up profraw and text files
		files, err = ioutil.ReadDir(covPath)
		for _, file := range files {
			if strings.HasSuffix(file.Name(), "profraw") ||
				strings.HasSuffix(file.Name(), "txt") {
				err = os.Remove(path.Join(covPath, file.Name()))
				if err != nil {
					log.Log.Error(err)
				}
			}
		}
	}

	finalResult.Summary.TaskTiming.EndPostprocess = time.Now()

	return finalResult, nil
}

// buildCovMapping merges existing profraw files and extracts the mapping from basic blocks to bit vector indices
// so we can build coverage files properly
func buildCovMapping(covPath string, profraws []string, infile string, outfile string) {
	cmd := exec.Command("llvm-profdata", append([]string{"merge",
		"--text", "--failure-mode=any", "--num-threads=1",
		"--output", path.Join(covPath, "full.txt")}, profraws...)...)

	err := cmd.Run()
	if err != nil {
		log.Log.Error("Could not run llvm-profdata successfully. It might not be in your path, or you might have passed" +
			"in corrupted .profraw files.")
		return
	}

	f, err := os.Open(path.Join(covPath, "full.txt"))
	if err != nil {
		log.Log.Error(err)
		return
	}
	defer f.Close()

	var g *os.File
	var writer *csv.Writer
	write := false
	if outfile != "" {
		g, err = os.Create(outfile)
		if err != nil {
			log.Log.Error(err)
			return
		}
		defer g.Close()
		writer = csv.NewWriter(g)
		write = true
	}

	var count int
	fn := ""
	m := make(map[string]int)

	nextIsCount := false

	count = 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if nextIsCount {
			numBlocks, err := strconv.Atoi(line)
			if err != nil {
				log.Log.Error("bad num blocks: ", line)
				return
			}
			count += numBlocks
			nextIsCount = false
		}

		if strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "# Num Counters:") {
				nextIsCount = true
			}
			continue
		}

		if strings.TrimSpace(line) == "" {
			fn = ""
			continue
		}

		if fn == "" {
			fn = strings.TrimSpace(line)
			m[fn] = count
			if write {
				err = writer.Write([]string{fn, strconv.Itoa(int(count))})
				if err != nil {
					log.Log.Error(err)
				}
			}
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		log.Log.Error(err)
		return
	}

	if write {
		err = writer.Write([]string{"END", strconv.Itoa(int(count))})
		if err != nil {
			log.Log.Error(err)
		}
		writer.Flush()
	}
	covMappingLength = count
	covMapping = m

	log.Log.Debugf("Created coverage mapping with %d functions and %d total blocks", len(m), covMappingLength)

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
