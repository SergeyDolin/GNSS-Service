package handlers

import (
	"io"
	"net/http"
)

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

func IndexHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Only GET request allowed!", http.StatusMethodNotAllowed)
		return
	}

	res.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Metrics</title></head>
<body><h1>GNSS Service (Collaborative positioning)</h1></body></html>`
	io.WriteString(res, html)
}
