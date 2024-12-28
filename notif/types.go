package notif

type AccountUpdateReplyMessage struct {
	Data struct {
		Code    int    `json:"code"`
		Account string `json:"account"`
		Message string `json:"message"`
	} `json:"data"`
}
