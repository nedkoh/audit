package main

import (  
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"

    "goji.io"
    "goji.io/pat"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
)

func ErrorWithJSON(w http.ResponseWriter, message string, code int) {  
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(code)
    fmt.Fprintf(w, "{message: %q}", message)
}

func ResponseWithJSON(w http.ResponseWriter, json []byte, code int) {  
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(code)
    w.Write(json)
}

type Event struct {
    Id   bson.ObjectId `json:"id" bson:"_id,omitempty"`
    //Id    string     `json:"id"` `bson:"_id,"` 
    Entity   string  `json:"entity"`
    Action  string   `json:"action"`
    Event   string   `json:"event"`
    Time time.Time   `json:"time"` 
    //Time    int32    `json:"time"`    
    Author string    `json:"author"`
}

func main() {  
    session, err := mgo.Dial("localhost")
    if err != nil {
        panic(err)
    }
    defer session.Close()

    session.SetMode(mgo.Monotonic, true)
    ensureIndex(session)

    mux := goji.NewMux()
    mux.HandleFunc(pat.Get("/event"), allEvents(session))
    mux.HandleFunc(pat.Post("/event"), addEvent(session))
    mux.HandleFunc(pat.Get("/event/:id"), eventById(session))
    mux.HandleFunc(pat.Put("/event/:id"), updateEvent(session))
    mux.HandleFunc(pat.Delete("/event/:id"), deleteEvent(session))
    http.ListenAndServe("localhost:8080", mux)
}

func ensureIndex(s *mgo.Session) {  
    session := s.Copy()
    defer session.Close()

    c := session.DB("audit").C("events")

    index := mgo.Index{
        Key:        []string{"Id"},
        Unique:     true,
        DropDups:   true,
        Background: true,
        Sparse:     true,
    }
    err := c.EnsureIndex(index)
    if err != nil {
        panic(err)
    }
}

func allEvents(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {  
    return func(w http.ResponseWriter, r *http.Request) {
        session := s.Copy()
        defer session.Close()

        c := session.DB("audit").C("events")

        var events []Event
        err := c.Find(bson.M{}).All(&events)
        if err != nil {
            ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
            log.Println("Failed get all events: ", err)
            return
        }

        respBody, err := json.MarshalIndent(events, "", "  ")
        if err != nil {
            log.Fatal(err)
        }

        ResponseWithJSON(w, respBody, http.StatusOK)
    }
}

func addEvent(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {  
    return func(w http.ResponseWriter, r *http.Request) {
        session := s.Copy()
        defer session.Close()

        var event Event
        decoder := json.NewDecoder(r.Body)
        err := decoder.Decode(&event)
        if err != nil {
            ErrorWithJSON(w, "Incorrect body" + err.Error(), http.StatusBadRequest)
            return
        }

        c := session.DB("audit").C("events")
        //event.Id = bson.NewObjectId()
        //log.Println("Created Id: ", event.Id)
        err = c.Insert(event)
        if err != nil {
            if mgo.IsDup(err) {
                ErrorWithJSON(w, "Event with this Id already exists", http.StatusBadRequest)
                return
            }

            ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
            log.Println("Failed insert event: ", err)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Location", r.URL.Path+"/"+ event.Id.Hex())
        w.WriteHeader(http.StatusCreated)
    }
}

func eventById(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {  
    return func(w http.ResponseWriter, r *http.Request) {
        session := s.Copy()
        defer session.Close()

        id := bson.ObjectIdHex(pat.Param(r, "id"))


        c := session.DB("audit").C("events")

        var event Event
        err := c.FindId(id).One(&event)
        if err != nil {
            ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
            log.Println("Failed find event: ", err)
            return
        }

        if event.Id == "" {
            ErrorWithJSON(w, "Event not found", http.StatusNotFound)
            return
        }

        respBody, err := json.MarshalIndent(event, "", "  ")
        if err != nil {
            log.Fatal(err)
        }

        ResponseWithJSON(w, respBody, http.StatusOK)
    }
}

func updateEvent(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {  
    return func(w http.ResponseWriter, r *http.Request) {
        session := s.Copy()
        defer session.Close()

        id := bson.ObjectIdHex(pat.Param(r, "id"))

        var event Event
        decoder := json.NewDecoder(r.Body)
        err := decoder.Decode(&event)
        if err != nil {
            ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
            return
        }

        c := session.DB("audit").C("events")

        err = c.UpdateId(id, &event)
        if err != nil {
            switch err {
            default:
                ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
                log.Println("Failed update event: ", err)
                return
            case mgo.ErrNotFound:
                ErrorWithJSON(w, "Event not found", http.StatusNotFound)
                return
            }
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

func deleteEvent(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {  
    return func(w http.ResponseWriter, r *http.Request) {
        session := s.Copy()
        defer session.Close()

        id := bson.ObjectIdHex(pat.Param(r, "id"))

        c := session.DB("audit").C("events")

        err := c.RemoveId(id)
        if err != nil {
            switch err {
            default:
                ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
                log.Println("Failed delete event: ", err)
                return
            case mgo.ErrNotFound:
                ErrorWithJSON(w, "Event not found", http.StatusNotFound)
                return
            }
        }

        w.WriteHeader(http.StatusNoContent)
    }
}
