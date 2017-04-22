package jsonconf

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
)

var (
	dQuto     = byte('"')
	slash     = byte('/')
	backslash = byte('\\')
	lineBreak = []byte{'\n'}
	comment   = []byte{slash, slash}
)

// ParseJSONFile parse config from file
func ParseJSONFile(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return ParseJSONData(data, v)
}

// ParseJSONData parse config by given data
func ParseJSONData(data []byte, v interface{}) error {
	data = StripJSONOneLineComments(data)
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	return nil
}

// StripJSONOneLineComments strip json data "//" comments
func StripJSONOneLineComments(data []byte) []byte {
	datas := bytes.Split(data, lineBreak)

	var slashNum = 0
	inEscape := false
	inDQuto := false

	for k, line := range datas {

		// reset
		slashNum = 0
		inEscape = false
		inDQuto = false

		line = bytes.TrimSpace(line)

		if cmtIdx := bytes.Index(line, comment); cmtIdx == -1 { // this line no comment
			continue
		} else if cmtIdx == 0 {
			datas[k] = lineBreak
			continue
		}

		for i := 0; i < len(line); i++ {
			if inEscape {
				inEscape = !inEscape
				continue // continue all escaped letter
			}
			switch line[i] {
			case backslash:
				inEscape = !inEscape
			case dQuto:
				inDQuto = !inDQuto
			}

			if line[i] == slash {
				slashNum++
			} else if slashNum > 0 {
				slashNum = 0
			}

			if slashNum == 2 && !inDQuto {
				datas[k] = line[:i-1]
				break
			}
		}
	}
	return bytes.Join(datas, lineBreak)
}
