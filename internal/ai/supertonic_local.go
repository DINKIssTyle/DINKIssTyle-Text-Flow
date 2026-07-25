//go:build darwin

package ai

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/text/unicode/norm"
)

// TTSModelStatus holds status for downloading models
type TTSModelStatus struct {
	IsDownloaded bool    `json:"isDownloaded"`
	Status       string  `json:"status"` // "idle", "downloading", "done", "error"
	Progress     float64 `json:"progress"`
	CurrentFile  string  `json:"currentFile"`
	Error        string  `json:"error,omitempty"`
}

// Available languages for multilingual TTS
var AvailableLangs = []string{"en", "ko", "ja", "ar", "bg", "cs", "da", "de", "el", "es", "et", "fi", "fr", "hi", "hr", "hu", "id", "it", "lt", "lv", "nl", "pl", "pt", "ro", "ru", "sk", "sl", "sv", "tr", "uk", "vi", "na"}

// Configuration structures matching supertonic3 tts.json
type SpecProcessorConfig struct {
	NFFT      int     `json:"n_fft"`
	WinLength int     `json:"win_length"`
	HopLength int     `json:"hop_length"`
	NMels     int     `json:"n_mels"`
	Eps       float64 `json:"eps"`
	NormMean  float64 `json:"norm_mean"`
	NormStd   float64 `json:"norm_std"`
}

type EncoderConfig struct {
	SpecProcessor SpecProcessorConfig `json:"spec_processor"`
}

type AEConfig struct {
	SampleRate    int           `json:"sample_rate"`
	BaseChunkSize int           `json:"base_chunk_size"`
	Encoder       EncoderConfig `json:"encoder"`
}

type StyleTokenLayerConfig struct {
	NStyle        int `json:"n_style"`
	StyleValueDim int `json:"style_value_dim"`
}

type StyleEncoderConfig struct {
	StyleTokenLayer StyleTokenLayerConfig `json:"style_token_layer"`
}

type ProjOutConfig struct {
	Idim int `json:"idim"`
	Odim int `json:"odim"`
}

type TextEncoderConfig struct {
	ProjOut ProjOutConfig `json:"proj_out"`
}

type TTLConfig struct {
	ChunkCompressFactor int                `json:"chunk_compress_factor"`
	LatentDim           int                `json:"latent_dim"`
	StyleEncoder        StyleEncoderConfig `json:"style_encoder"`
	TextEncoder         TextEncoderConfig  `json:"text_encoder"`
}

type DPStyleEncoderConfig struct {
	StyleTokenLayer StyleTokenLayerConfig `json:"style_token_layer"`
}

type DPConfig struct {
	LatentDim           int                  `json:"latent_dim"`
	ChunkCompressFactor int                  `json:"chunk_compress_factor"`
	StyleEncoder        DPStyleEncoderConfig `json:"style_encoder"`
}

type Config struct {
	AE  AEConfig  `json:"ae"`
	TTL TTLConfig `json:"ttl"`
	DP  DPConfig  `json:"dp"`
}

// VoiceStyleData holds voice style JSON structure
type VoiceStyleData struct {
	StyleTTL struct {
		Data [][][]float64 `json:"data"`
		Dims []int64       `json:"dims"`
		Type string        `json:"type"`
	} `json:"style_ttl"`
	StyleDP struct {
		Data [][][]float64 `json:"data"`
		Dims []int64       `json:"dims"`
		Type string        `json:"type"`
	} `json:"style_dp"`
}

// UnicodeProcessor processes characters to index ids
type UnicodeProcessor struct {
	indexer []int64
}

func NewUnicodeProcessor(unicodeIndexerPath string) (*UnicodeProcessor, error) {
	data, err := os.ReadFile(unicodeIndexerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read unicode indexer file: %w", err)
	}

	var indexer []int64
	if err := json.Unmarshal(data, &indexer); err != nil {
		return nil, fmt.Errorf("failed to parse unicode indexer JSON: %w", err)
	}

	return &UnicodeProcessor{indexer: indexer}, nil
}

func (up *UnicodeProcessor) Call(textList []string, langList []string) ([][]int64, [][][]float64) {
	processedTexts := make([]string, len(textList))
	for i, text := range textList {
		processedTexts[i] = preprocessText(text, langList[i])
	}

	textLengths := make([]int64, len(processedTexts))
	maxLen := 0
	for i, text := range processedTexts {
		textLengths[i] = int64(len([]rune(text)))
		if int(textLengths[i]) > maxLen {
			maxLen = int(textLengths[i])
		}
	}

	textIDs := make([][]int64, len(processedTexts))
	for i, text := range processedTexts {
		row := make([]int64, maxLen)
		runes := []rune(text)
		for j, r := range runes {
			unicodeVal := int(r)
			if unicodeVal < len(up.indexer) {
				row[j] = up.indexer[unicodeVal]
			} else {
				row[j] = -1
			}
		}
		textIDs[i] = row
	}

	textMask := lengthToMask(textLengths, maxLen)
	return textIDs, textMask
}

// Style holds style tensors
type Style struct {
	TtlTensor *ort.Tensor[float32]
	DpTensor  *ort.Tensor[float32]
}

func (s *Style) Destroy() {
	if s.TtlTensor != nil {
		s.TtlTensor.Destroy()
	}
	if s.DpTensor != nil {
		s.DpTensor.Destroy()
	}
}

func LoadVoiceStyle(path string) (*Style, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read voice style JSON: %w", err)
	}

	var styleData VoiceStyleData
	if err := json.Unmarshal(data, &styleData); err != nil {
		return nil, fmt.Errorf("failed to parse voice style JSON: %w", err)
	}

	ttlDims := styleData.StyleTTL.Dims
	dpDims := styleData.StyleDP.Dims

	ttlSize := 1
	for _, dim := range ttlDims {
		ttlSize *= int(dim)
	}
	dpSize := 1
	for _, dim := range dpDims {
		dpSize *= int(dim)
	}

	ttlFlat := make([]float32, ttlSize)
	idx := 0
	for _, batch := range styleData.StyleTTL.Data {
		for _, row := range batch {
			for _, val := range row {
				ttlFlat[idx] = float32(val)
				idx++
			}
		}
	}

	dpFlat := make([]float32, dpSize)
	idx = 0
	for _, batch := range styleData.StyleDP.Data {
		for _, row := range batch {
			for _, val := range row {
				dpFlat[idx] = float32(val)
				idx++
			}
		}
	}

	ttlTensor, err := ort.NewTensor(ttlDims, ttlFlat)
	if err != nil {
		return nil, fmt.Errorf("failed to create TTL style tensor: %w", err)
	}

	dpTensor, err := ort.NewTensor(dpDims, dpFlat)
	if err != nil {
		ttlTensor.Destroy()
		return nil, fmt.Errorf("failed to create DP style tensor: %w", err)
	}

	return &Style{
		TtlTensor: ttlTensor,
		DpTensor:  dpTensor,
	}, nil
}

// SupertonicEngine represents the locally loaded TTS models
type SupertonicEngine struct {
	cfg           Config
	textProcessor *UnicodeProcessor
	dpOrt         *ort.DynamicAdvancedSession
	textEncOrt    *ort.DynamicAdvancedSession
	vectorEstOrt  *ort.DynamicAdvancedSession
	vocoderOrt    *ort.DynamicAdvancedSession
	SampleRate    int
	baseChunkSize int
	chunkCompress int
	ldim          int
}

var (
	ortOnce    sync.Once
	ortInitErr error
)

func InitONNXRuntime(supertonicDir string) error {
	ortOnce.Do(func() {
		dylibPath := filepath.Join(supertonicDir, "runtime", "libonnxruntime.dylib")
		ort.SetSharedLibraryPath(dylibPath)
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

func LoadSupertonicEngine(supertonicDir string) (*SupertonicEngine, error) {
	if err := InitONNXRuntime(supertonicDir); err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	onnxDir := filepath.Join(supertonicDir, "onnx")

	// Load Config
	cfgPath := filepath.Join(onnxDir, "tts.json")
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tts config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse tts config: %w", err)
	}

	// Load dynamic sessions
	dpPath := filepath.Join(onnxDir, "duration_predictor.onnx")
	textEncPath := filepath.Join(onnxDir, "text_encoder.onnx")
	vectorEstPath := filepath.Join(onnxDir, "vector_estimator.onnx")
	vocoderPath := filepath.Join(onnxDir, "vocoder.onnx")

	dpOrt, err := ort.NewDynamicAdvancedSession(dpPath, []string{"text_ids", "style_dp", "text_mask"}, []string{"duration"}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load duration predictor session: %w", err)
	}

	textEncOrt, err := ort.NewDynamicAdvancedSession(textEncPath, []string{"text_ids", "style_ttl", "text_mask"}, []string{"text_emb"}, nil)
	if err != nil {
		dpOrt.Destroy()
		return nil, fmt.Errorf("failed to load text encoder session: %w", err)
	}

	vectorEstOrt, err := ort.NewDynamicAdvancedSession(vectorEstPath,
		[]string{"noisy_latent", "text_emb", "style_ttl", "latent_mask", "text_mask", "current_step", "total_step"},
		[]string{"denoised_latent"}, nil)
	if err != nil {
		dpOrt.Destroy()
		textEncOrt.Destroy()
		return nil, fmt.Errorf("failed to load vector estimator session: %w", err)
	}

	vocoderOrt, err := ort.NewDynamicAdvancedSession(vocoderPath, []string{"latent"}, []string{"wav_tts"}, nil)
	if err != nil {
		dpOrt.Destroy()
		textEncOrt.Destroy()
		vectorEstOrt.Destroy()
		return nil, fmt.Errorf("failed to load vocoder session: %w", err)
	}

	unicodeIndexerPath := filepath.Join(onnxDir, "unicode_indexer.json")
	textProcessor, err := NewUnicodeProcessor(unicodeIndexerPath)
	if err != nil {
		dpOrt.Destroy()
		textEncOrt.Destroy()
		vectorEstOrt.Destroy()
		vocoderOrt.Destroy()
		return nil, err
	}

	return &SupertonicEngine{
		cfg:           cfg,
		textProcessor: textProcessor,
		dpOrt:         dpOrt,
		textEncOrt:    textEncOrt,
		vectorEstOrt:  vectorEstOrt,
		vocoderOrt:    vocoderOrt,
		SampleRate:    cfg.AE.SampleRate,
		baseChunkSize: cfg.AE.BaseChunkSize,
		chunkCompress: cfg.TTL.ChunkCompressFactor,
		ldim:          cfg.TTL.LatentDim,
	}, nil
}

func (tts *SupertonicEngine) Destroy() {
	if tts.dpOrt != nil {
		tts.dpOrt.Destroy()
	}
	if tts.textEncOrt != nil {
		tts.textEncOrt.Destroy()
	}
	if tts.vectorEstOrt != nil {
		tts.vectorEstOrt.Destroy()
	}
	if tts.vocoderOrt != nil {
		tts.vocoderOrt.Destroy()
	}
}

func (tts *SupertonicEngine) sampleNoisyLatent(durOnnx []float32) ([][][]float64, [][][]float64) {
	bsz := len(durOnnx)
	maxDur := float64(0)
	for _, d := range durOnnx {
		if float64(d) > maxDur {
			maxDur = float64(d)
		}
	}

	wavLenMax := maxDur * float64(tts.SampleRate)
	wavLengths := make([]int64, bsz)
	for i, d := range durOnnx {
		wavLengths[i] = int64(float64(d) * float64(tts.SampleRate))
	}

	chunkSize := tts.baseChunkSize * tts.chunkCompress
	latentLen := int((wavLenMax + float64(chunkSize) - 1) / float64(chunkSize))
	latentDim := tts.ldim * tts.chunkCompress

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	noisyLatent := make([][][]float64, bsz)
	for b := 0; b < bsz; b++ {
		batch := make([][]float64, latentDim)
		for d := 0; d < latentDim; d++ {
			row := make([]float64, latentLen)
			for t := 0; t < latentLen; t++ {
				const eps = 1e-10
				u1 := math.Max(eps, rng.Float64())
				u2 := rng.Float64()
				row[t] = math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)
			}
			batch[d] = row
		}
		noisyLatent[b] = batch
	}

	latentMask := getLatentMask(wavLengths, tts.cfg)
	for b := 0; b < bsz; b++ {
		for d := 0; d < latentDim; d++ {
			for t := 0; t < latentLen; t++ {
				noisyLatent[b][d][t] *= latentMask[b][0][t]
			}
		}
	}

	return noisyLatent, latentMask
}

func (tts *SupertonicEngine) _infer(textList []string, langList []string, style *Style, totalStep int, speed float32) ([]float32, []float32, error) {
	bsz := len(textList)

	// Process text
	textIDs, textMask := tts.textProcessor.Call(textList, langList)
	textIDsShape := []int64{int64(bsz), int64(len(textIDs[0]))}
	textMaskShape := []int64{int64(bsz), 1, int64(len(textMask[0][0]))}

	textIDsTensor := IntArrayToTensor(textIDs, textIDsShape)
	defer textIDsTensor.Destroy()
	textMaskTensor := ArrayToTensor(textMask, textMaskShape)
	defer textMaskTensor.Destroy()

	// Predict duration
	dpOutputs := []ort.Value{nil}
	err := tts.dpOrt.Run(
		[]ort.Value{textIDsTensor, style.DpTensor, textMaskTensor},
		dpOutputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run duration predictor: %w", err)
	}
	durTensor := dpOutputs[0].(*ort.Tensor[float32])
	defer durTensor.Destroy()
	durOnnx := durTensor.GetData()

	for i := range durOnnx {
		durOnnx[i] /= speed
	}

	// Encode text
	textIDsTensor2 := IntArrayToTensor(textIDs, textIDsShape)
	defer textIDsTensor2.Destroy()
	textEncOutputs := []ort.Value{nil}
	err = tts.textEncOrt.Run(
		[]ort.Value{textIDsTensor2, style.TtlTensor, textMaskTensor},
		textEncOutputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run text encoder: %w", err)
	}
	textEmbTensor := textEncOutputs[0].(*ort.Tensor[float32])
	defer textEmbTensor.Destroy()

	// Sample noisy latent
	xt, latentMask := tts.sampleNoisyLatent(durOnnx)
	latentShape := []int64{int64(bsz), int64(len(xt[0])), int64(len(xt[0][0]))}
	latentMaskShape := []int64{int64(bsz), 1, int64(len(latentMask[0][0]))}

	totalStepArray := make([]float32, bsz)
	for b := 0; b < bsz; b++ {
		totalStepArray[b] = float32(totalStep)
	}
	scalarShape := []int64{int64(bsz)}

	totalStepTensor, _ := ort.NewTensor(scalarShape, totalStepArray)
	defer totalStepTensor.Destroy()

	// Denoising loop
	for step := 0; step < totalStep; step++ {
		currentStepArray := make([]float32, bsz)
		for b := 0; b < bsz; b++ {
			currentStepArray[b] = float32(step)
		}

		currentStepTensor, _ := ort.NewTensor(scalarShape, currentStepArray)
		noisyLatentTensor := ArrayToTensor(xt, latentShape)
		latentMaskTensor := ArrayToTensor(latentMask, latentMaskShape)
		textMaskTensor2 := ArrayToTensor(textMask, textMaskShape)

		vectorEstOutputs := []ort.Value{nil}
		err = tts.vectorEstOrt.Run(
			[]ort.Value{noisyLatentTensor, textEmbTensor, style.TtlTensor, latentMaskTensor, textMaskTensor2,
				currentStepTensor, totalStepTensor},
			vectorEstOutputs,
		)
		if err != nil {
			currentStepTensor.Destroy()
			noisyLatentTensor.Destroy()
			latentMaskTensor.Destroy()
			textMaskTensor2.Destroy()
			return nil, nil, fmt.Errorf("failed to run vector estimator: %w", err)
		}

		denoisedTensor := vectorEstOutputs[0].(*ort.Tensor[float32])
		denoisedData := denoisedTensor.GetData()

		idx := 0
		for b := 0; b < bsz; b++ {
			for d := 0; d < len(xt[b]); d++ {
				for t := 0; t < len(xt[b][d]); t++ {
					xt[b][d][t] = float64(denoisedData[idx])
					idx++
				}
			}
		}

		noisyLatentTensor.Destroy()
		latentMaskTensor.Destroy()
		textMaskTensor2.Destroy()
		currentStepTensor.Destroy()
		denoisedTensor.Destroy()
	}

	// Generate waveform
	finalLatentTensor := ArrayToTensor(xt, latentShape)
	defer finalLatentTensor.Destroy()

	vocoderOutputs := []ort.Value{nil}
	err = tts.vocoderOrt.Run(
		[]ort.Value{finalLatentTensor},
		vocoderOutputs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run vocoder: %w", err)
	}

	wavBatchTensor := vocoderOutputs[0].(*ort.Tensor[float32])
	defer wavBatchTensor.Destroy()
	wav := wavBatchTensor.GetData()

	return wav, durOnnx, nil
}

func (tts *SupertonicEngine) Synthesize(text string, lang string, style *Style, totalStep int, speed float32) ([]float32, error) {
	maxLen := 300
	if lang == "ko" || lang == "ja" {
		maxLen = 120
	}
	chunks := chunkText(text, maxLen)

	var wavCat []float32
	silenceDuration := float32(0.3)

	for i, chunk := range chunks {
		wav, duration, err := tts._infer([]string{chunk}, []string{lang}, style, totalStep, speed)
		if err != nil {
			return nil, err
		}

		dur := duration[0]
		wavLen := int(float32(tts.SampleRate) * dur)
		if wavLen > len(wav) {
			wavLen = len(wav)
		}
		wavChunk := wav[:wavLen]

		if i == 0 {
			wavCat = wavChunk
		} else {
			silenceLen := int(silenceDuration * float32(tts.SampleRate))
			silence := make([]float32, silenceLen)

			wavCat = append(wavCat, silence...)
			wavCat = append(wavCat, wavChunk...)
		}
	}

	return wavCat, nil
}

// WriteWav writes the float32 audio data to a 16-bit PCM WAV file
func WriteWav(filename string, audioData []float32, sampleRate int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	numSamples := len(audioData)
	numChannels := 1
	bitsPerSample := 16
	byteRate := sampleRate * numChannels * (bitsPerSample / 8)
	blockAlign := numChannels * (bitsPerSample / 8)
	dataSize := numSamples * numChannels * (bitsPerSample / 8)
	chunkSize := 36 + dataSize

	// Write RIFF header
	_, _ = file.Write([]byte("RIFF"))
	_ = writeInt32(file, int32(chunkSize))
	_, _ = file.Write([]byte("WAVE"))

	// Write fmt subchunk
	_, _ = file.Write([]byte("fmt "))
	_ = writeInt32(file, 16)
	_ = writeInt16(file, 1) // PCM format
	_ = writeInt16(file, int16(numChannels))
	_ = writeInt32(file, int32(sampleRate))
	_ = writeInt32(file, int32(byteRate))
	_ = writeInt16(file, int16(blockAlign))
	_ = writeInt16(file, int16(bitsPerSample))

	// Write data subchunk
	_, _ = file.Write([]byte("data"))
	_ = writeInt32(file, int32(dataSize))

	// Write PCM audio data (convert float32 to int16)
	for _, sample := range audioData {
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		val := int16(sample * 32767.0)
		_ = writeInt16(file, val)
	}

	return nil
}

func writeInt32(w io.Writer, val int32) error {
	return binary.Write(w, binary.LittleEndian, val)
}

func writeInt16(w io.Writer, val int16) error {
	return binary.Write(w, binary.LittleEndian, val)
}

// Downloader State Management
var (
	downloadMu     sync.Mutex
	downloadStatus = TTSModelStatus{
		Status: "idle",
	}
	downloadCancel context.CancelFunc
)

const TotalDownloadBytes = 417542601

var fileExpectedSizes = map[string]int64{
	"duration_predictor.onnx":               3700147,
	"text_encoder.onnx":                     36416150,
	"vector_estimator.onnx":                 256534781,
	"vocoder.onnx":                          101424195,
	"unicode_indexer.json":                  277676,
	"tts.json":                              8253,
	"voice_styles/M1.json":                  291748,
	"voice_styles/M2.json":                  292055,
	"voice_styles/M3.json":                  290198,
	"voice_styles/M4.json":                  291522,
	"voice_styles/M5.json":                  291469,
	"voice_styles/F1.json":                  292046,
	"voice_styles/F2.json":                  292423,
	"voice_styles/F3.json":                  290794,
	"voice_styles/F4.json":                  291808,
	"voice_styles/F5.json":                  291479,
	"onnxruntime-osx-universal2-1.18.1.tgz": 16265857,
}

func CheckModelStatus(supertonicDir string) TTSModelStatus {
	downloadMu.Lock()
	defer downloadMu.Unlock()

	// If currently downloading/error, return that status
	if downloadStatus.Status == "downloading" || downloadStatus.Status == "error" {
		return downloadStatus
	}

	onnxDir := filepath.Join(supertonicDir, "onnx")
	voiceStylesDir := filepath.Join(supertonicDir, "voice_styles")
	runtimeDir := filepath.Join(supertonicDir, "runtime")

	// 1. Verify models exist
	models := []string{"duration_predictor.onnx", "text_encoder.onnx", "vector_estimator.onnx", "vocoder.onnx", "unicode_indexer.json", "tts.json"}
	for _, m := range models {
		if !fileIsNonEmpty(filepath.Join(onnxDir, m)) {
			return TTSModelStatus{IsDownloaded: false, Status: "idle"}
		}
	}

	// 2. Verify voice styles exist
	styles := []string{"M1.json", "M2.json", "M3.json", "M4.json", "M5.json", "F1.json", "F2.json", "F3.json", "F4.json", "F5.json"}
	for _, s := range styles {
		if !fileIsNonEmpty(filepath.Join(voiceStylesDir, s)) {
			return TTSModelStatus{IsDownloaded: false, Status: "idle"}
		}
	}

	// 3. Verify ONNX Runtime library exists
	if !fileIsNonEmpty(filepath.Join(runtimeDir, "libonnxruntime.dylib")) {
		return TTSModelStatus{IsDownloaded: false, Status: "idle"}
	}

	downloadStatus = TTSModelStatus{IsDownloaded: true, Status: "done", Progress: 100}
	return downloadStatus
}

func fileIsNonEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func CancelTTSModelDownload() {
	downloadMu.Lock()
	defer downloadMu.Unlock()

	if downloadCancel != nil {
		downloadCancel()
		downloadCancel = nil
	}
	downloadStatus = TTSModelStatus{
		IsDownloaded: false,
		Status:       "idle",
		Progress:     0,
	}
}

func StartTTSModelDownload(supertonicDir string, onProgress func(TTSModelStatus)) error {
	downloadMu.Lock()
	defer downloadMu.Unlock()

	if downloadStatus.Status == "downloading" {
		return errors.New("download is already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	downloadCancel = cancel
	downloadStatus = TTSModelStatus{
		IsDownloaded: false,
		Status:       "downloading",
		Progress:     0,
	}

	go runDownloader(ctx, supertonicDir, onProgress)
	return nil
}

func runDownloader(ctx context.Context, supertonicDir string, onProgress func(TTSModelStatus)) {
	onnxDir := filepath.Join(supertonicDir, "onnx")
	voiceStylesDir := filepath.Join(supertonicDir, "voice_styles")
	runtimeDir := filepath.Join(supertonicDir, "runtime")

	// Ensure directories exist
	_ = os.MkdirAll(onnxDir, 0755)
	_ = os.MkdirAll(voiceStylesDir, 0755)
	_ = os.MkdirAll(runtimeDir, 0755)

	var totalWritten int64
	updateProgress := func(currentFile string, written int64) {
		downloadMu.Lock()
		totalWritten += written
		progress := float64(totalWritten) * 100.0 / float64(TotalDownloadBytes)
		if progress > 99.9 {
			progress = 99.9 // Keep at 99.9 until fully verified
		}
		downloadStatus.Progress = progress
		downloadStatus.CurrentFile = currentFile
		statusCopy := downloadStatus
		downloadMu.Unlock()
		onProgress(statusCopy)
	}

	setError := func(errStr string) {
		downloadMu.Lock()
		downloadStatus.Status = "error"
		downloadStatus.Error = errStr
		statusCopy := downloadStatus
		downloadMu.Unlock()
		onProgress(statusCopy)
	}

	// 1. Download models
	hfBase := "https://huggingface.co/Supertone/supertonic-3/resolve/main/"
	models := []string{"duration_predictor.onnx", "text_encoder.onnx", "vector_estimator.onnx", "vocoder.onnx", "unicode_indexer.json", "tts.json"}
	for _, m := range models {
		url := hfBase + "onnx/" + m
		dest := filepath.Join(onnxDir, m)
		err := downloadFile(ctx, url, dest, m, updateProgress)
		if err != nil {
			if ctx.Err() != nil {
				return // Canceled
			}
			setError(fmt.Sprintf("Failed to download model %s: %v", m, err))
			return
		}
	}

	// 2. Download voice styles
	styles := []string{"M1.json", "M2.json", "M3.json", "M4.json", "M5.json", "F1.json", "F2.json", "F3.json", "F4.json", "F5.json"}
	for _, s := range styles {
		url := hfBase + "voice_styles/" + s
		dest := filepath.Join(voiceStylesDir, s)
		err := downloadFile(ctx, url, dest, "voice_styles/"+s, updateProgress)
		if err != nil {
			if ctx.Err() != nil {
				return // Canceled
			}
			setError(fmt.Sprintf("Failed to download voice style %s: %v", s, err))
			return
		}
	}

	// 3. Download ONNX Runtime archive and extract dylib
	runtimeURL := "https://github.com/microsoft/onnxruntime/releases/download/v1.18.1/onnxruntime-osx-universal2-1.18.1.tgz"
	dylibDest := filepath.Join(runtimeDir, "libonnxruntime.dylib")

	err := downloadAndExtractDylib(ctx, runtimeURL, dylibDest, updateProgress)
	if err != nil {
		if ctx.Err() != nil {
			return // Canceled
		}
		setError(fmt.Sprintf("Failed to download or extract ONNX Runtime dylib: %v", err))
		return
	}

	// Finish
	downloadMu.Lock()
	downloadStatus.Status = "done"
	downloadStatus.Progress = 100
	downloadStatus.IsDownloaded = true
	downloadStatus.CurrentFile = ""
	statusCopy := downloadStatus
	downloadMu.Unlock()
	onProgress(statusCopy)
}

func downloadFile(ctx context.Context, url string, dest string, fileName string, updateProgress func(string, int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	buffer := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				_ = os.Remove(dest)
				return writeErr
			}
			updateProgress(fileName, int64(n))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = os.Remove(dest)
			return err
		}
	}

	return nil
}

func downloadAndExtractDylib(ctx context.Context, url string, dest string, updateProgress func(string, int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	progressReader := &progressTrackingReader{
		r: resp.Body,
		onRead: func(n int64) {
			updateProgress("onnxruntime-osx-universal2-1.18.1.tgz", n)
		},
	}

	gr, err := gzip.NewReader(progressReader)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg && strings.Contains(header.Name, "libonnxruntime") && strings.HasSuffix(header.Name, ".dylib") {
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, tr)
			if err != nil {
				_ = os.Remove(dest)
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("libonnxruntime library not found in tar.gz archive")
}

type progressTrackingReader struct {
	r      io.Reader
	onRead func(int64)
}

func (pr *progressTrackingReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.onRead(int64(n))
	}
	return n, err
}

// Local TTS Helpers
func preprocessText(text string, lang string) string {
	text = norm.NFKD.String(text)

	// Remove emojis
	emojiPattern := regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{1F700}-\x{1F77F}\x{1F780}-\x{1F7FF}\x{1F800}-\x{1F8FF}\x{1F900}-\x{1F9FF}\x{1FA00}-\x{1FA6F}\x{1FA70}-\x{1FAFF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}\x{1F1E6}-\x{1F1FF}]+`)
	text = emojiPattern.ReplaceAllString(text, "")

	// Replace dashes and symbols
	replacements := map[string]string{
		"–":      "-",
		"‑":      "-",
		"—":      "-",
		"_":      " ",
		"\u201C": "\"",
		"\u201D": "\"",
		"\u2018": "'",
		"\u2019": "'",
		"´":      "'",
		"`":      "'",
		"[":      " ",
		"]":      " ",
		"|":      " ",
		"/":      " ",
		"#":      " ",
		"→":      " ",
		"←":      " ",
	}
	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	specialSymbols := []string{"♥", "☆", "♡", "©", "\\"}
	for _, symbol := range specialSymbols {
		text = strings.ReplaceAll(text, symbol, "")
	}

	exprReplacements := map[string]string{
		"@":     " at ",
		"e.g.,": "for example, ",
		"i.e.,": "that is, ",
	}
	for old, new := range exprReplacements {
		text = strings.ReplaceAll(text, old, new)
	}

	// Spacing punctuation
	text = regexp.MustCompile(` ,`).ReplaceAllString(text, ",")
	text = regexp.MustCompile(` \.`).ReplaceAllString(text, ".")
	text = regexp.MustCompile(` !`).ReplaceAllString(text, "!")
	text = regexp.MustCompile(` \?`).ReplaceAllString(text, "?")
	text = regexp.MustCompile(` ;`).ReplaceAllString(text, ";")
	text = regexp.MustCompile(` :`).ReplaceAllString(text, ":")
	text = regexp.MustCompile(` '`).ReplaceAllString(text, "'")

	for strings.Contains(text, `""`) {
		text = strings.ReplaceAll(text, `""`, `"`)
	}
	for strings.Contains(text, "''") {
		text = strings.ReplaceAll(text, "''", "'")
	}
	for strings.Contains(text, "``") {
		text = strings.ReplaceAll(text, "``", "`")
	}

	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	if text != "" {
		endsWithPunct := regexp.MustCompile(`[.!?;:,'"\x{201C}\x{201D}\x{2018}\x{2019})\]}…。」』】〉》›»]$`)
		if !endsWithPunct.MatchString(text) {
			text += "."
		}
	}

	valid := false
	for _, l := range AvailableLangs {
		if l == lang {
			valid = true
			break
		}
	}
	if !valid {
		lang = "na"
	}

	return fmt.Sprintf("<%s>%s</%s>", lang, text, lang)
}

func lengthToMask(lengths []int64, maxLen int) [][][]float64 {
	bsz := len(lengths)
	mask := make([][][]float64, bsz)
	for i := 0; i < bsz; i++ {
		row := make([]float64, maxLen)
		for j := 0; j < maxLen; j++ {
			if int64(j) < lengths[i] {
				row[j] = 1.0
			} else {
				row[j] = 0.0
			}
		}
		mask[i] = [][]float64{row}
	}
	return mask
}

func getLatentMask(wavLengths []int64, cfg Config) [][][]float64 {
	baseChunkSize := int64(cfg.AE.BaseChunkSize)
	chunkCompressFactor := int64(cfg.TTL.ChunkCompressFactor)
	latentSize := baseChunkSize * chunkCompressFactor

	latentLengths := make([]int64, len(wavLengths))
	maxLen := int64(0)
	for i, wavLen := range wavLengths {
		latentLengths[i] = (wavLen + latentSize - 1) / latentSize
		if latentLengths[i] > maxLen {
			maxLen = latentLengths[i]
		}
	}
	return lengthToMask(latentLengths, int(maxLen))
}

const maxChunkLength = 300

var abbreviations = []string{
	"Dr.", "Mr.", "Mrs.", "Ms.", "Prof.", "Sr.", "Jr.",
	"St.", "Ave.", "Rd.", "Blvd.", "Dept.", "Inc.", "Ltd.",
	"Co.", "Corp.", "etc.", "vs.", "i.e.", "e.g.", "Ph.D.",
}

func chunkText(text string, maxLen int) []string {
	if maxLen == 0 {
		maxLen = maxChunkLength
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	var chunks []string

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if len(para) <= maxLen {
			chunks = append(chunks, para)
			continue
		}

		sentences := splitSentences(para)
		var current strings.Builder
		currentLen := 0

		for _, sentence := range sentences {
			sentence = strings.TrimSpace(sentence)
			if sentence == "" {
				continue
			}

			sentenceLen := len(sentence)
			if sentenceLen > maxLen {
				if current.Len() > 0 {
					chunks = append(chunks, strings.TrimSpace(current.String()))
					current.Reset()
					currentLen = 0
				}

				parts := strings.Split(sentence, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}

					partLen := len(part)
					if partLen > maxLen {
						words := strings.Fields(part)
						var wordChunk strings.Builder
						wordChunkLen := 0

						for _, word := range words {
							wordLen := len(word)
							if wordChunkLen+wordLen+1 > maxLen && wordChunk.Len() > 0 {
								chunks = append(chunks, strings.TrimSpace(wordChunk.String()))
								wordChunk.Reset()
								wordChunkLen = 0
							}

							if wordChunk.Len() > 0 {
								wordChunk.WriteString(" ")
								wordChunkLen++
							}
							wordChunk.WriteString(word)
							wordChunkLen += wordLen
						}

						if wordChunk.Len() > 0 {
							chunks = append(chunks, strings.TrimSpace(wordChunk.String()))
						}
					} else {
						if currentLen+partLen+1 > maxLen && current.Len() > 0 {
							chunks = append(chunks, strings.TrimSpace(current.String()))
							current.Reset()
							currentLen = 0
						}

						if current.Len() > 0 {
							current.WriteString(", ")
							currentLen += 2
						}
						current.WriteString(part)
						currentLen += partLen
					}
				}
				continue
			}

			if currentLen+sentenceLen+1 > maxLen && current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
				current.Reset()
				currentLen = 0
			}

			if current.Len() > 0 {
				current.WriteString(" ")
				currentLen++
			}
			current.WriteString(sentence)
			currentLen += sentenceLen
		}

		if current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
		}
	}

	if len(chunks) == 0 {
		return []string{""}
	}
	return chunks
}

func splitSentences(text string) []string {
	re := regexp.MustCompile(`([.!?])\s+`)
	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return []string{text}
	}

	var sentences []string
	lastEnd := 0

	for _, match := range matches {
		beforePunc := text[lastEnd:match[0]]
		isAbbrev := false
		for _, abbrev := range abbreviations {
			if strings.HasSuffix(strings.TrimSpace(beforePunc+text[match[0]:match[0]+1]), abbrev) {
				isAbbrev = true
				break
			}
		}

		if !isAbbrev {
			sentences = append(sentences, text[lastEnd:match[1]])
			lastEnd = match[1]
		}
	}

	if lastEnd < len(text) {
		sentences = append(sentences, text[lastEnd:])
	}

	if len(sentences) == 0 {
		return []string{text}
	}
	return sentences
}

func ArrayToTensor(array [][][]float64, shape []int64) *ort.Tensor[float32] {
	totalSize := int64(1)
	for _, dim := range shape {
		totalSize *= dim
	}

	flat := make([]float32, totalSize)
	idx := 0
	for b := 0; b < len(array); b++ {
		for d := 0; d < len(array[b]); d++ {
			for t := 0; t < len(array[b][d]); t++ {
				flat[idx] = float32(array[b][d][t])
				idx++
			}
		}
	}

	tensor, err := ort.NewTensor(shape, flat)
	if err != nil {
		panic(err)
	}
	return tensor
}

func IntArrayToTensor(array [][]int64, shape []int64) *ort.Tensor[int64] {
	totalSize := int64(1)
	for _, dim := range shape {
		totalSize *= dim
	}

	flat := make([]int64, totalSize)
	idx := 0
	for b := 0; b < len(array); b++ {
		for t := 0; t < len(array[b]); t++ {
			flat[idx] = array[b][t]
			idx++
		}
	}

	tensor, err := ort.NewTensor(shape, flat)
	if err != nil {
		panic(err)
	}
	return tensor
}
