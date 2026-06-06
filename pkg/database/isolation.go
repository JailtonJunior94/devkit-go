package database

import "strconv"

type IsolationLevel int

const (
	LevelDefault IsolationLevel = iota
	LevelReadUncommitted
	LevelReadCommitted
	LevelWriteCommitted
	LevelRepeatableRead
	LevelSnapshot
	LevelSerializable
	LevelLinearizable
)

func (l IsolationLevel) String() string {
	switch l {
	case LevelDefault:
		return "LevelDefault"
	case LevelReadUncommitted:
		return "LevelReadUncommitted"
	case LevelReadCommitted:
		return "LevelReadCommitted"
	case LevelWriteCommitted:
		return "LevelWriteCommitted"
	case LevelRepeatableRead:
		return "LevelRepeatableRead"
	case LevelSnapshot:
		return "LevelSnapshot"
	case LevelSerializable:
		return "LevelSerializable"
	case LevelLinearizable:
		return "LevelLinearizable"
	default:
		return "IsolationLevel(" + strconv.Itoa(int(l)) + ")"
	}
}
