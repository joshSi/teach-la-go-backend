package db

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/uclaacm/teach-la-go-backend/httpext"
	"golang.org/x/net/websocket"
)

// Session describes a collaborative coding environment.
type Session struct {
	// Conns maps names to their corresponding
	// connections.
	Conns map[string]*websocket.Conn `json:"conns"`
}

// Maps session UUIDs to maps of UIDs to websocket
// connections.
var sessions map[string]Session

// CreateCollab creates a collaborative editing
// session resident in memory.
//
// Body:
// {
//   "uid": string
// }
//
// Response: New session UUID
//
// If no users are present 5 minutes from its
// creation, it is destroyed.
func (d *DB) CreateCollab(c echo.Context) error {
	// Body must properly bind to a user object.
	owner := User{}
	if err := httpext.RequestBodyTo(c.Request(), &owner); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	// Create the session.
	uid := uuid.New().String()
	sessions[uid] = Session{
		Conns: make(map[string]*websocket.Conn),
	}

	// Delete the session if users are not present
	// 5 minutes from creation.
	go func() {
		time.Sleep(5 * time.Minute)
		if session, ok := sessions[uid]; ok && len(session.Conns) == 0 {
			delete(sessions, uid)
		}
	}()

	// Return the session UUID.
	return c.String(http.StatusCreated, uid)
}

// JoinCollab attempts to join the provided user
// to the given session ID.
//
// Body:
// {
//   "uid": string
// }
func (d *DB) JoinCollab(c echo.Context) error {
	// Check for valid body type.
	var body struct {
		UID string `json:"uid"`
	}
	if err := httpext.RequestBodyTo(c.Request(), &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	// Check for session UUID passed as an Echo
	// path parameter.
	uuid := c.Param("uuid")
	if _, ok := sessions[uuid]; !ok {
		return c.String(http.StatusBadRequest, "provided UUID is invalid.")
	}

	// Change protocol to websocket connection.
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		// TODO: handshake

		// Add user to session.
		session := sessions[uuid]
		session.Conns[body.UID] = ws

		// Begin event loop.
		for {
			// Read some message.
			msg := ""
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				// Failure to read shall be interpreted as
				// connection failure.
				c.Logger().Error(err)
				delete(session.Conns, body.UID)
			}

			// Echo it back.
			if err := websocket.Message.Send(ws, msg); err != nil {
				c.Logger().Error(err)
			}
		}
	}).ServeHTTP(c.Response(), c.Request())

	return nil
}
