package model

type Media struct {
	Type       string `json:"type"` // 媒体类型：image, video, voice, file
	Key        string `json:"key"`  // MD5
	Path       string `json:"path"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Data       []byte `json:"data"` // for voice
	ModifyTime int64  `json:"modifyTime"`
}

