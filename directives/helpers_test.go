package directives

import (
	"github.com/herdworks/hypertemplates-go/data"
	"github.com/herdworks/hypertemplates-go/parse"
)

type TemplateData = data.TemplateData
type TemplateDataSet = data.TemplateDataSet

func readTemplateDataFile(path string) (TemplateData, error) {
	return parse.ReadFile(path)
}
