package main

// ApiResponse описывает структуру ответа от API Фонбета
type ApiResponse struct {
	Events []Event `json:"events"`
	// ... добавь поля по необходимости
}

// Event — одно событие
type Event struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parentId"`
	Name      string `json:"name"`
	SportID   int64  `json:"sportId"`
	StartTime int64  `json:"startTime"`
	Place     string `json:"place"`
	Priority  int    `json:"priority"`
	// ... добавь нужные поля
}
