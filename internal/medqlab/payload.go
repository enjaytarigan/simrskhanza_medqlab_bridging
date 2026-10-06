package medqlab

import "encoding/json"

// Envelope is the MedQLab push/getResult body (see MEDQLAB.md).
type Envelope struct {
	Response *Response         `json:"response"`
	MetaData *MetaData         `json:"metaData"`
	Raw      json.RawMessage   `json:"-"`
}

type MetaData struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type Response struct {
	Count          int           `json:"count"`
	NoLaboratorium string        `json:"noLaboratorium"`
	NoOrder        string        `json:"noOrder"`
	NoPendaftaran  string        `json:"no_pendaftaran"` // SIMRS no_rawat (if echoed at response root)
	Demographics   *Demographics `json:"demographics"`
	Examinations   []Examination `json:"examinations"`
	Orders         string        `json:"orders"`
	Counter        *Counter      `json:"counter"`
}

type Demographics struct {
	RegNumber        string  `json:"regNumber"`
	VisitNumber      string  `json:"visitNumber"`  // SIMRS no_rawat (preferred)
	VisitNumberSnake string  `json:"visit_number"` // alternate key
	NoOrder          string  `json:"noOrder"`
	CollectDate      string  `json:"collectDate"` // specimen collection time → permintaan_lab.tgl_sampel/jam_sampel
	CommentsSample   *string `json:"commentsSample"`
	Diagnose         string  `json:"diagnose"`
	SourceType       string  `json:"sourceType"`
}

type Counter struct {
	TotalExam          int `json:"totalExam"`
	TotalExamValidated int `json:"totalExamValidated"`
	TotalExamVerified  int `json:"totalExamVerified"`
}

// Examination is a node in the hierarchical MedQLab tree.
type Examination struct {
	TestID               json.Number   `json:"testId"`
	Type                 string        `json:"type"`
	ParentID             *json.Number  `json:"parentId"`
	Position             string        `json:"position"`
	TestName             string        `json:"testName"`
	EnglishName          string        `json:"englishName"`
	ExternalID           string        `json:"externalId"`
	AliasCode            *string       `json:"aliasCode"`
	LocalCode            *string       `json:"localCode"`
	DepartmentType       string        `json:"departmentType"`
	ExamValue            *string       `json:"examValue"`
	ExamValueType        *string       `json:"examValueType"`
	ExamValueFlag        *string       `json:"examValueFlag"`
	NormalValueText      *string       `json:"normalValueText"`
	UnitName             *string       `json:"unitName"`
	MethodName           *string       `json:"methodName"`
	OriginalResult       *string       `json:"originalResult"`
	ResultInterpretation *string       `json:"resultInterpretation"`
	ValidatedAt          *string       `json:"validatedAt"`
	VerifiedAt           *string       `json:"verifiedAt"`
	ValidatedUsername    *string       `json:"validatedUsername"`
	IdEmployeeVerify     *string       `json:"idEmployeeVerify"`
	Children             []Examination `json:"children"`
}

// LeafResult is a flattened analyte row ready for SIMRS mapping.
type LeafResult struct {
	LisTestID        string
	LocalCode        string
	TestName         string
	Position         string
	Nilai            string
	Keterangan       string
	NilaiRujukan     string
	ValidatedAt      string
	VerifiedAt       string
	IdEmployeeVerify string
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
