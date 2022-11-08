package main

type Hub struct {
	roomId     string
	owner      string
	readOnly   bool //房间模式，false代表协作模式，true代表只读模式，只有房主可以更改房间的模式，只读模式下只有房主可以操作白板
	clients    map[*Client]bool
	broadcast  chan []byte
	unregister chan *Client
}

func newHub(roomId string) *Hub {
	return &Hub{
		roomId:     roomId,
		broadcast:  make(chan []byte),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	defer func() {
		close(h.unregister)
		close(h.broadcast)
	}()
	for {
		select {
		case client := <-h.unregister:
			h.OnDisconnect(client)
		case message := <-h.broadcast:
			h.BroadcastToRoom(message)
		}
	}
}

func (h *Hub) OnDisconnect(client *Client) {
	roomMutex := roomMutexes[h.roomId]
	roomMutex.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		//房间无人时从房间列表删去该房间
		if len(h.clients) == 0 {
			house.Delete(h.roomId)
			roomMutex.Unlock()
			mutexForRoomMutexes.Lock()
			if roomMutex.TryLock() {
				if len(h.clients) == 0 {
					delete(roomMutexes, h.roomId)
				}
				roomMutex.Unlock()
			}
			mutexForRoomMutexes.Unlock()
			return
		}

		//房间还剩一人时其自动成为房主
		if len(h.clients) == 1 {
			for c, _ := range h.clients {
				h.owner = c.id
			}
			return
		}

		//房主离开时房间里的人自动成为房主
		if client.id == h.owner {
			for c, _ := range h.clients {
				h.owner = c.id
				break
			}
		}
	}
	roomMutex.Unlock()
}

func (h *Hub) BroadcastToRoom(message []byte) {
	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}
