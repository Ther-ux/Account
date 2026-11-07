// Package ledger defines ledger records and their validation rules.
package ledger

import (
	"errors"
	"time"
)

// Record stores an income or expense amount in integer cents.
type Record struct {
	Date        string `json:"date"`
	Type        string `json:"type"`
	AmountCents int64  `json:"amountCents"`
	Category    string `json:"category"`
	Note        string `json:"note"`
}

// Validate checks a record without modifying it or performing any I/O.
// JSON decoding, including rejecting fractional or overflowing cents, belongs
// to the caller. Note is optional and is preserved as supplied.
func Validate(record Record) error {
	date, err := time.Parse(time.DateOnly, record.Date)
	// Parse checks the calendar; the round trip also enforces zero padding.
	if err != nil || date.Year() < 1 || date.Format(time.DateOnly) != record.Date {
		return errors.New("date 必须为真实的 YYYY-MM-DD 日期")
	}
	if record.AmountCents <= 0 {
		return errors.New("amountCents 必须为正整数，单位为分")
	}

	switch record.Type {
	case "income":
		switch record.Category {
		case "工资", "奖金", "其他收入":
			return nil
		default:
			return errors.New("income 的 category 必须为工资、奖金或其他收入")
		}
	case "expense":
		switch record.Category {
		case "饮食", "交通", "购物", "其他支出":
			return nil
		default:
			return errors.New("expense 的 category 必须为饮食、交通、购物或其他支出")
		}
	default:
		return errors.New("type 必须为 income 或 expense")
	}
}
