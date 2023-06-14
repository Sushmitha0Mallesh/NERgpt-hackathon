package main

import "time"

type (
	Span struct {
		Offset int `json:"offset"`
		Length int `json:"length"`
	}

	BoundingRegion struct {
		PageNumber int   `json:"pageNumber"`
		Polygon    []int `json:"polygon"`
	}

	Paragraph struct {
		Spans           []Span           `json:"spans"`
		BoundingRegions []BoundingRegion `json:"boundingRegions"`
		Content         string           `json:"content"`
	}

	Style struct {
		Confidence    float64 `json:"confidence"`
		Spans         []Span  `json:"spans"`
		IsHandwritten bool    `json:"isHandwritten"`
	}

	Language struct {
		Spans      []Span  `json:"spans"`
		Locale     string  `json:"locale"`
		Confidence float64 `json:"confidence"`
	}

	AnalyzeResult struct {
		APIVersion      string `json:"apiVersion"`
		ModelID         string `json:"modelId"`
		StringIndexType string `json:"stringIndexType"`
		Content         string `json:"content"`
		Pages           []struct {
			PageNumber int     `json:"pageNumber"`
			Angle      float64 `json:"angle"`
			Width      int     `json:"width"`
			Height     int     `json:"height"`
			Unit       string  `json:"unit"`
			Words      []struct {
				Content    string  `json:"content"`
				Polygon    []int   `json:"polygon"`
				Confidence float64 `json:"confidence"`
				Span       struct {
					Offset int `json:"offset"`
					Length int `json:"length"`
				} `json:"span"`
			} `json:"words"`
			Lines []struct {
				Content string `json:"content"`
				Polygon []int  `json:"polygon"`
				Spans   []struct {
					Offset int `json:"offset"`
					Length int `json:"length"`
				} `json:"spans"`
			} `json:"lines"`
			Spans []struct {
				Offset int `json:"offset"`
				Length int `json:"length"`
			} `json:"spans"`
			Kind string `json:"kind"`
		} `json:"pages"`
		Paragraphs []Paragraph `json:"paragraphs"`
		Styles     []Style     `json:"styles"`
		Languages  []Language  `json:"languages"`
	}

	OCRAnalysisResult struct {
		Status              string        `json:"status"`
		CreatedDateTime     time.Time     `json:"createdDateTime"`
		LastUpdatedDateTime time.Time     `json:"lastUpdatedDateTime"`
		AnalyzeResult       AnalyzeResult `json:"analyzeResult"`
	}

	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	Choice struct {
		Index        int     `json:"index"`
		FinishReason string  `json:"finish_reason"`
		Message      Message `json:"message"`
	}

	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		PromptTokens     int `json:"prompt_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}

	GPTResult struct {
		ID      string   `json:"id"`
		Object  string   `json:"object"`
		Created int      `json:"created"`
		Model   string   `json:"model"`
		Choices []Choice `json:"choices"`
		Usage   `json:"usage"`
	}
)
