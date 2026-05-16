package service

type DataPipelineStepCatalogEntry struct {
	Type     string
	Category string
	Order    int
}

func DataPipelineStepCatalog() []DataPipelineStepCatalogEntry {
	return append([]DataPipelineStepCatalogEntry(nil), dataPipelineStepCatalogEntries...)
}

func dataPipelineStepCatalogMap() map[string]struct{} {
	out := make(map[string]struct{}, len(dataPipelineStepCatalogEntries))
	for _, entry := range dataPipelineStepCatalogEntries {
		out[entry.Type] = struct{}{}
	}
	return out
}

var dataPipelineStepCatalogEntries = []DataPipelineStepCatalogEntry{
	{Type: DataPipelineStepInput, Category: "input_output", Order: 0},
	{Type: DataPipelineStepExtractText, Category: "extraction", Order: 12},
	{Type: DataPipelineStepJSONExtract, Category: "extraction", Order: 13},
	{Type: DataPipelineStepExcelExtract, Category: "extraction", Order: 14},
	{Type: DataPipelineStepClassifyDocument, Category: "extraction", Order: 15},
	{Type: DataPipelineStepExtractFields, Category: "extraction", Order: 16},
	{Type: DataPipelineStepExtractTable, Category: "extraction", Order: 18},
	{Type: DataPipelineStepProductExtraction, Category: "extraction", Order: 19},
	{Type: DataPipelineStepDetectLanguage, Category: "quality", Order: 19},
	{Type: DataPipelineStepProfile, Category: "transform", Order: 10},
	{Type: DataPipelineStepClean, Category: "quality", Order: 20},
	{Type: DataPipelineStepCanonicalize, Category: "quality", Order: 22},
	{Type: DataPipelineStepRedactPII, Category: "quality", Order: 24},
	{Type: DataPipelineStepDeduplicate, Category: "quality", Order: 26},
	{Type: DataPipelineStepNormalize, Category: "quality", Order: 30},
	{Type: DataPipelineStepValidate, Category: "quality", Order: 40},
	{Type: DataPipelineStepSchemaMapping, Category: "schema", Order: 50},
	{Type: DataPipelineStepSchemaCompletion, Category: "schema", Order: 60},
	{Type: DataPipelineStepSchemaInference, Category: "schema", Order: 62},
	{Type: DataPipelineStepUnion, Category: "transform", Order: 66},
	{Type: DataPipelineStepJoin, Category: "transform", Order: 68},
	{Type: DataPipelineStepEnrichJoin, Category: "transform", Order: 70},
	{Type: DataPipelineStepEntityResolution, Category: "transform", Order: 72},
	{Type: DataPipelineStepUnitConversion, Category: "transform", Order: 74},
	{Type: DataPipelineStepRelationship, Category: "transform", Order: 76},
	{Type: DataPipelineStepTransform, Category: "transform", Order: 80},
	{Type: DataPipelineStepPartitionFilter, Category: "transform", Order: 84},
	{Type: DataPipelineStepWatermarkFilter, Category: "transform", Order: 85},
	{Type: DataPipelineStepSnapshotSCD2, Category: "transform", Order: 86},
	{Type: DataPipelineStepConfidenceGate, Category: "quality", Order: 90},
	{Type: DataPipelineStepQuarantine, Category: "quality", Order: 91},
	{Type: DataPipelineStepHumanReview, Category: "quality", Order: 92},
	{Type: DataPipelineStepRouteByCondition, Category: "quality", Order: 93},
	{Type: DataPipelineStepSampleCompare, Category: "quality", Order: 94},
	{Type: DataPipelineStepQualityReport, Category: "quality", Order: 96},
	{Type: DataPipelineStepOutput, Category: "input_output", Order: 1000},
}
