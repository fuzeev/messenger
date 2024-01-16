package main

import (
	"database/sql"
	"log"
	"net/http"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

var (
	db      *sql.DB
	err     error
	clients = make(map[*websocket.Conn]bool) // Подключенные клиенты
	mutex   sync.Mutex                       // Для безопасного доступа к клиентам
)

// Инициализация подключения к базе данных
func initDB() {
	db, err = sql.Open("mysql", "login:pass@tcp(localhost:3306)/gotest")
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
}

// Структура для сообщения
type Message struct {
	UserID  int
	Content string
	Time    time.Time
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Рассылка сообщения всем подключенным клиентам
func broadcastMessage(message Message) {
	mutex.Lock()
	defer mutex.Unlock()
	for client := range clients {
		err := client.WriteJSON(message)
		if err != nil {
			log.Printf("error: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// Обработчик WebSocket соединений
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Регистрация нового клиента
	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}

		// Сохранение сообщения в базе данных и рассылка его всем клиентам
		_, err = db.Exec("INSERT INTO messages (user_id, text, datetime) VALUES (?, ?, ?)", msg.UserID, msg.Content, msg.Time)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}

		broadcastMessage(msg)
	}
}

func main() {
	// Инициализация подключения к базе данных
	initDB()
	defer db.Close()

	// Настройка маршрута
	http.HandleFunc("/ws", handleConnections)

	// Запуск сервера
	log.Println("http server started on :8000")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
