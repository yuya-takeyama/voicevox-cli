package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/oto"
)

type Params struct {
	AccentPhrases      []AccentPhrases `json:"accent_phrases"`
	SpeedScale         float64         `json:"speedScale"`
	PitchScale         float64         `json:"pitchScale"`
	IntonationScale    float64         `json:"intonationScale"`
	VolumeScale        float64         `json:"volumeScale"`
	PrePhonemeLength   float64         `json:"prePhonemeLength"`
	PostPhonemeLength  float64         `json:"postPhonemeLength"`
	OutputSamplingRate int             `json:"outputSamplingRate"`
	OutputStereo       bool            `json:"outputStereo"`
	Kana               string          `json:"kana"`
}

type Mora struct {
	Text            string   `json:"text"`
	Consonant       *string  `json:"consonant"`
	ConsonantLength *float64 `json:"consonant_length"`
	Vowel           string   `json:"vowel"`
	VowelLength     float64  `json:"vowel_length"`
	Pitch           float64  `json:"pitch"`
}

type AccentPhrases struct {
	Moras           []Mora `json:"moras"`
	Accent          int    `json:"accent"`
	PauseMora       *Mora  `json:"pause_mora"`
	IsInterrogative bool   `json:"is_interrogative"`
}

type Speakers []struct {
	Name        string   `json:"name"`
	SpeakerUUID string   `json:"speaker_uuid"`
	Styles      []Styles `json:"styles"`
	Version     string   `json:"version"`
}

type Styles struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type config struct {
	endpoint   string
	speaker    int
	style      int
	speed      float64
	intonation float64
	volume     float64
	pitch      float64
	output     string
}

var audioDir string

type CacheKeyPair struct {
	Key   string
	Value string
}

type CacheKey []CacheKeyPair

func (c CacheKey) Len() int           { return len(c) }
func (c CacheKey) Less(i, j int) bool { return c[i].Key < c[j].Key }
func (c CacheKey) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

func generateCacheKey(text string, speakerID int, cfg config) string {
	cacheKey := CacheKey{
		{Key: "text", Value: text},
		{Key: "speakerID", Value: strconv.Itoa(speakerID)},
		{Key: "speaker", Value: strconv.Itoa(cfg.speaker)},
		{Key: "style", Value: strconv.Itoa(cfg.style)},
		{Key: "speed", Value: strconv.FormatFloat(cfg.speed, 'f', -1, 64)},
		{Key: "intonation", Value: strconv.FormatFloat(cfg.intonation, 'f', -1, 64)},
		{Key: "volume", Value: strconv.FormatFloat(cfg.volume, 'f', -1, 64)},
		{Key: "pitch", Value: strconv.FormatFloat(cfg.pitch, 'f', -1, 64)},
	}

	sort.Sort(cacheKey)

	jsonData, _ := json.Marshal(cacheKey)
	hasher := sha1.New()
	hasher.Write(jsonData)
	return hex.EncodeToString(hasher.Sum(nil))
}

func getCacheFilePath(hash string) string {
	return filepath.Join(audioDir, hash+".wav")
}

func checkCacheDir() error {
	info, err := os.Stat(audioDir)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return err
	}
	if !info.IsDir() {
		return os.ErrInvalid
	}
	return nil
}

func getSpeakers(cfg config) Speakers {
	resp, err := http.Get(cfg.endpoint + "/speakers")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	var speakers Speakers
	if err := json.NewDecoder(resp.Body).Decode(&speakers); err != nil {
		log.Fatal(err)
	}
	return speakers
}

func getQuery(cfg config, id int, text string) (*Params, error) {
	req, err := http.NewRequest("POST", cfg.endpoint+"/audio_query", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("speaker", strconv.Itoa(id))
	q.Add("text", text)
	req.URL.RawQuery = q.Encode()
	//log.Println(req.URL.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var params *Params
	if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
		return nil, err
	}
	return params, nil
}

func synth(cfg config, id int, params *Params) ([]byte, error) {
	b, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return nil, err
	}
	//log.Println(string(b))
	req, err := http.NewRequest("POST", cfg.endpoint+"/synthesis", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "audio/wav")
	req.Header.Add("Content-Type", "application/json")
	q := req.URL.Query()
	q.Add("speaker", strconv.Itoa(id))
	req.URL.RawQuery = q.Encode()
	//log.Println(req.URL.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buff := bytes.NewBuffer(nil)
	if _, err := io.Copy(buff, resp.Body); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func playback(params *Params, b []byte) error {
	ch := 1
	if params.OutputStereo {
		ch = 2
	}
	ctx, err := oto.NewContext(params.OutputSamplingRate, ch, 2, 3200)
	if err != nil {
		return err
	}
	defer ctx.Close()
	p := ctx.NewPlayer()
	if _, err := io.Copy(p, bytes.NewReader(b)); err != nil {
		return err
	}
	if err := p.Close(); err != nil {
		return err
	}
	return nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	
	// Set audio directory from environment variable or use default
	audioDir = os.Getenv("VOICEVOX_CLI_AUDIO_DIR")
	if audioDir == "" {
		audioDir = "audio"
	}
	
	cfg := config{}
	flag.StringVar(&cfg.endpoint, "endpoint", "http://localhost:50021", "api endpoint")
	flag.IntVar(&cfg.speaker, "speaker", 0, "speaker")
	flag.StringVar(&cfg.output, "o", "", "output wav file")
	flag.IntVar(&cfg.style, "style", 0, "style")
	flag.Float64Var(&cfg.speed, "speed", 1.0, "speed")
	flag.Float64Var(&cfg.intonation, "intonation", 1.0, "intonation")
	flag.Float64Var(&cfg.volume, "volume", 1.0, "volume")
	flag.Float64Var(&cfg.pitch, "pitch", 0.0, "pitch")
	flag.Parse()
	
	if err := checkCacheDir(); err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Audio cache directory does not exist: %s", audioDir)
		} else if err == os.ErrInvalid {
			log.Fatalf("%s is not a directory", audioDir)
		} else {
			log.Fatalf("Failed to check audio cache directory: %v", err)
		}
	}
	
	speakers := getSpeakers(cfg)
	if cfg.speaker >= len(speakers) {
		log.Fatal("speaker not found")
	}
	spk := speakers[cfg.speaker]
	if cfg.style >= len(spk.Styles) {
		log.Fatal("style not found")
	}
	spkID := spk.Styles[cfg.style].ID
	log.Println(spk.Name, spk.Styles[cfg.style].Name, spkID)
	
	text := strings.Join(flag.Args(), " ")
	cacheHash := generateCacheKey(text, spkID, cfg)
	cacheFilePath := getCacheFilePath(cacheHash)
	
	params, err := getQuery(cfg, spkID, text)
	if err != nil {
		log.Fatal(err)
	}
	params.SpeedScale = cfg.speed
	params.PitchScale = cfg.pitch
	params.IntonationScale = cfg.intonation
	params.VolumeScale = cfg.volume
	
	var b []byte
	if _, err := os.Stat(cacheFilePath); err == nil {
		log.Println("Using cached audio:", cacheHash)
		b, err = os.ReadFile(cacheFilePath)
		if err != nil {
			log.Fatal("Failed to read cache file:", err)
		}
	} else {
		log.Println("Generating new audio")
		b, err = synth(cfg, spkID, params)
		if err != nil {
			log.Fatal(err)
		}
		
		if err := os.WriteFile(cacheFilePath, b, 0644); err != nil {
			log.Println("Failed to write cache file:", err)
		}
	}
	
	if len(cfg.output) > 0 {
		if err := os.WriteFile(cfg.output, b, 0644); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := playback(params, b[44:]); err != nil {
			log.Fatal(err)
		}
	}
}
