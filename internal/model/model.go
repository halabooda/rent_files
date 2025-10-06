package model

type Profile struct {
	Id int `json:"id"`
}

type MediaRecord struct {
	Src  string `json:"src"`
	Type string `json:"type"`
}
