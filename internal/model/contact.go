package model

type Contact struct {
	UserName string `json:"userName"`
	Alias    string `json:"alias"`
	Remark   string `json:"remark"`
	NickName string `json:"nickName"`
	IsFriend bool   `json:"isFriend"`
}

func (c *Contact) DisplayName() string {
	switch {
	case c.Remark != "":
		return c.Remark
	case c.NickName != "":
		return c.NickName
	}
	return ""
}
