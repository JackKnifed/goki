// this file contains a pre-processor to pull some stuff out of the markdown file before parsing it

package gnosis

import (
	"bufio"
	"io"
	"os"
)

type pageMetadata struct {
	Keywords []string
	Topics   []string
	Page     []byte
}

var pdata pageMetadata

func (pdata *PageMetadata) lineIsTitle(line []byte) bool {
	finalLength := len(line)
	i := 0

	// if the row doesn't start with tabs, spaces, or ='s
	if (data[i] != ' ' && data[i] != '=') && data[i] != '\t' {
		return false
	}

	// skip any spaces or tabs at the start
	for data[i] == ' ' || data[i] == '\t' {
		i++
	}

	// if the next item's not a =, bail out
	if data[i] != '=' {
		return false
	}

	// run through all of the ='s
	for data[i] == '=' {
		i++
	}

	if data[i] != ' ' && data[i] != '\t' && data[i] != '\n' {
		return false
	}

	//ditch all spaces after any ='s
	for data[i] == ' ' || data[i] == '\t' {
		i++
	}

	if finalLength == i+1 {
		return true
	} else {
		return false
	}
}

func (pdata *PageMetadata) checkMatch(input []byte, looking []byte, tracker []string) {
	// trim off any blank spaces at the start of the line
	value := bytes.Trim(line, " \t")

	if input[:len(looking)] == looking {
		// trim off the target from the []byte
		value = input[len(looking):]

		// trim spaces at the start and at the end
		value = bytes.Trim(value, " \t\n")

		if value[0] == ':' || value[0] == '=' {
			value = bytes.Trim(value, " \t\n=:")
		}

		// replace any spaces in the middle with -'s, and suppress double spaces
		bytes.Replace(value, " ", "-", -1)

		for i := 1; i < len(value); i++ {
			if value[i-1] == '-' && value[1] == '-' {
				value = value[:i] + value[i+1:]
			}
		}

		// now just add the value to the array that you're tracking
		tracker.Append(string(value))
	}
}

func (pdata *PageMetadata) ProcessMetadata(line []byte) error {
	checkMatch(line, "tag=", pdata.Topics)
	checkMatch(line, "tag =", pdata.Topics)
	checkMatch(line, "topic=", pdata.Topics)
	checkMatch(line, "topic =", pdata.Topics)
	checkMatch(line, "category=", pdata.Topics)
	checkMatch(line, "category =", pdata.Topics)

	checkMatch(line, "keyword=", pdata.Keywords)
	checkMatch(line, "keyword =", pdata.Keywords)
	checkMatch(line, "keywords=", pdata.Keywords)
	checkMatch(line, "keywords =", pdata.Keywords)
	checkMatch(line, "meta=", pdata.Keywords)
	checkMatch(line, "meta =", pdata.Keywords)
}

func (pdata *PageMetadata) LoadPage(pageName string) error {
	f, err := os.Open(pageName)
	reader := bufio.NewReader(f)
	upperLine, fullLine, err := reader.ReadLine()

	// inspect the first line you read
	if err != nil {
		return err
	} else if !fullLine {
		return err.New("first line I read wasn't a full line")
	} else if lineIsTitle(upperLine) {
		return err.New("first line looks an awful lot like the underside of the title o.O")
	}

	lowerLine, fullLine, err := reader.ReadLine()

	// inspect the lower line
	if err != nil {
		return err
	} else if !fullLine {
		return err.New("second line I read wasn't a full line")
	} else if lineIsTitle(lowerLine) {
		// if the second line is a title, read the rest of the page in
		// you don't have any metadata to work with here, move on
		upperLine.Append('\n')
		upperLine.Append(lowerLine)
		upperLine.Append('\n')

		_, err = reader.Read(lowerLine)
		if err != nil {
			return err
		}
	}

	// if you're at this point, the first line is metadata
	// you gotta process it and work with the next line
	// so let's just read through the file until we hit the title
	for !lineIsTitle(lowerLine) {
		// process the line
		ProcessMetadata(upperLine)
		// shift the lower line up
		upperLine = lowerLine
		// read in a new lower line
		lowerLine, fullLine, err := reader.ReadLine()
		if err != nil {
			return err
		} else if !fullLine {
			return err.New("I filled my buffer with a line")
		}
	}

	// by this point, I should have read everything in - let's read the rest and just return it
	upperLine.Append('\n')
	upperLine.Append(lowerLine)
	upperLine.Append('\n')

	_, err = reader.Read(lowerLine)
	return err
}