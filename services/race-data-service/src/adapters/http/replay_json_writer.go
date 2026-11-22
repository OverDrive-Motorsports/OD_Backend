/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_json_writer.go - Writes the replay dump JSON object part by part.
##
*/

package httpadapter

import (
	"fmt"
	"io"
	"strconv"

	"overdrive/services/race-data-service/src/core/domain"
)

// replayJSONWriter assembles the GET /race/replay object from parts arriving in dump order:
// every key of domain.ReplayDatasets is always written, a driver-scoped dataset with no part as
// `{}` and a track-wide one as `[]`, so the output matches the former single json.Encode.
type replayJSONWriter struct {
	out    io.Writer
	cursor int  // index in domain.ReplayDatasets of the dataset being written
	open   bool // the cursor dataset's key has been written
	empty  bool // the open dataset has no part yet
	err    error
}

// newReplayJSONWriter writes the opening brace on the first part or on finish.
func newReplayJSONWriter(out io.Writer) *replayJSONWriter {
	return &replayJSONWriter{out: out}
}

// write appends one part. Parts must follow domain.ReplayDatasets order.
func (j *replayJSONWriter) write(part domain.ReplayPart) error {
	index := replayDatasetIndex(part.Dataset)
	if index < j.cursor {
		return fmt.Errorf("replay part %q out of order", part.Dataset)
	}
	for j.cursor < index {
		j.closeDataset()
	}
	j.openDataset()
	dataset := domain.ReplayDatasets[j.cursor]
	if dataset.DriverScoped {
		if part.DriverNumber == nil {
			return fmt.Errorf("replay part %q has no driver", part.Dataset)
		}
		if !j.empty {
			j.put(",")
		}
		j.put(strconv.Quote(strconv.Itoa(*part.DriverNumber)) + ":")
	} else if !j.empty {
		return fmt.Errorf("track-wide replay dataset %q sent twice", part.Dataset)
	}
	j.putBytes(part.Rows)
	j.empty = false
	return j.err
}

// finish writes the datasets that received no part and closes the object.
func (j *replayJSONWriter) finish() error {
	for j.cursor < len(domain.ReplayDatasets) {
		j.closeDataset()
	}
	j.put("}\n")
	return j.err
}

// openDataset writes the key of the cursor dataset (and the opening brace of the whole object
// before the first one).
func (j *replayJSONWriter) openDataset() {
	if j.open {
		return
	}
	if j.cursor == 0 {
		j.put("{")
	} else {
		j.put(",")
	}
	dataset := domain.ReplayDatasets[j.cursor]
	j.put(strconv.Quote(dataset.Name) + ":")
	if dataset.DriverScoped {
		j.put("{")
	}
	j.open, j.empty = true, true
}

// closeDataset ends the cursor dataset (writing it empty if it got no part) and moves on.
func (j *replayJSONWriter) closeDataset() {
	j.openDataset()
	if domain.ReplayDatasets[j.cursor].DriverScoped {
		j.put("}")
	} else if j.empty {
		j.put("[]")
	}
	j.open = false
	j.cursor++
}

func (j *replayJSONWriter) put(text string) {
	j.putBytes([]byte(text))
}

func (j *replayJSONWriter) putBytes(data []byte) {
	if j.err != nil {
		return
	}
	_, j.err = j.out.Write(data)
}

// replayDatasetIndex returns the position of name in domain.ReplayDatasets, or -1.
func replayDatasetIndex(name string) int {
	for index, dataset := range domain.ReplayDatasets {
		if dataset.Name == name {
			return index
		}
	}
	return -1
}
