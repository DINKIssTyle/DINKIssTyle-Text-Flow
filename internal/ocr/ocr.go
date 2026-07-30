package ocr

const (
	LanguageAutomatic = "auto"

	ResultActionClipboard = "clipboard"
	ResultActionShow      = "show"
)

type Result struct {
	Text string `json:"text"`
}
