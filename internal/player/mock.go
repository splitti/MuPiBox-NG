package player

import "log"

type MockPlayer struct{}

func NewMockPlayer() *MockPlayer {
	return &MockPlayer{}
}

func (m *MockPlayer) Play(id string) error {
	log.Println("MOCK ▶ Play:", id)
	return nil
}

func (m *MockPlayer) Pause() error {
	log.Println("MOCK ⏸ Pause")
	return nil
}

func (m *MockPlayer) Next() error {
	log.Println("MOCK ⏭ Next")
	return nil
}

func (m *MockPlayer) SetShuffle(on bool) error {
	log.Println("MOCK 🔀 Shuffle:", on)
	return nil
}

http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Mock-Werte (später echt)
	w.Write([]byte(`{
		"mode": "kids",
		"time": "12:34",
		"battery": 82,
		"wifi": true,
		"volume": 45
	}`))
})
