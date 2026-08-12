package directives

import (
	"github.com/herdworks/httt/data"
	"github.com/herdworks/httt/parse"
)

type TemplateData = data.TemplateData
type TemplateDataSet = data.TemplateDataSet

func readTemplateDataFile(path string) (TemplateData, error) {
	return parse.ReadFile(path)
}
