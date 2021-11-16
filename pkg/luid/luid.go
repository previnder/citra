package luid

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ID is an array of 12 bytes that is composed of 8 bytes of time and 4 random
// bytes in big endian byte order.
type ID [12]byte

// New returns a new ID and the time.Time (current time in UTC) used for the
// first 8 bytes of ID.
//
// Remember to call rand.Seed before using New.
func New() (ID, time.Time) {
	now := time.Now().UTC()
	var id ID

	binary.BigEndian.PutUint64(id[:8], uint64(now.UnixNano()))
	binary.BigEndian.PutUint32(id[8:], rand.Uint32())
	return id, now
}

// FromString unmarshals hex and returns an ID.
//
// hex can be any valid hexadecimal number of len(ID) bytes. FromString does
// not check whether hex corrosponds to a valid ID.
func FromString(hex string) (id ID, err error) {
	err = id.UnmarshalText([]byte(hex))
	return
}

// EqualsTo returns true if d and x are identical byte strings.
func (d ID) EqualsTo(x ID) bool {
	for i := 0; i < len(d); i++ {
		if d[i] != x[i] {
			return false
		}
	}
	return true
}

// Zero sets all bytes of ID to 0.
func (d *ID) Zero() {
	for i := 0; i < len(d); i++ {
		d[i] = 0
	}
}

// String returns the hexadecimal encoding of ID.
func (d ID) String() string {
	return hex.EncodeToString(d[:])
}

// MarshalText implements encoding.TextMarshaler interface. And returns the
// hexadecimal encoding of ID.
func (d ID) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler interface. text is
// supposed to be the hexadecimal encoding of an ID.
func (d *ID) UnmarshalText(text []byte) error {
	n, err := hex.Decode(d[:], text)
	if err != nil {
		return err
	}
	if n != len(d) {
		return fmt.Errorf("ID unmarshaling fail (src is %v bytes)", n)
	}
	return nil
}

// Scan implements sql.Scanner interface.
func (d *ID) Scan(src interface{}) error {
	if src == nil {
		return errors.New("ID scan error: src is nil")
	}

	v, ok := src.([]byte)
	if !ok {
		return errors.New("ID scan error: src is of unknown type")
	}

	if len(v) != len(d) {
		return fmt.Errorf("ID scan error: value is %v bytes", len(v))
	}
	if n := copy(d[:], v); n != len(d) {
		return errors.New("ID scan error: failed to copy all bytes")
	}
	return nil
}

// Value implements driver.Valuer interface.
func (d ID) Value() (driver.Value, error) {
	return d[:], nil
}

// NullID represents an ID that may be null. It is identical to sql.NullString.
type NullID struct {
	ID    ID
	Valid bool
}

// Scan implements the sql.Scanner interface.
func (ni *NullID) Scan(src interface{}) error {
	if src == nil {
		ni.Valid = false
		ni.ID.Zero()
		return nil
	}
	ni.Valid = true
	return ni.ID.Scan(src)
}

// Value implements driver.Valuer interface.
func (ni NullID) Value() (driver.Value, error) {
	if !ni.Valid {
		return nil, nil
	}
	return ni.ID.Value()
}

// MarshalJSON implements json.Marshalar interface.
func (ni NullID) MarshalJSON() ([]byte, error) {
	if ni.Valid {
		return json.Marshal(ni.ID.String())
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements json.Unmarshalar interface.
func (ni *NullID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		ni.Valid = false
		ni.ID.Zero()
		return nil
	}

	err := ni.ID.UnmarshalText(b[1 : len(b)-1])
	ni.Valid = err == nil
	return err
}
