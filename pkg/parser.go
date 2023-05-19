package rsvz_checker

import (
	"errors"
	"strconv"
	"strings"
)

type ParsedResult struct {
	Code     int    `json:"code"`
	Phone    int    `json:"phone"`
	FullNum  string `json:"full_num"`
	Operator string `json:"operator"`
	Region   string `json:"region"`
}

var ErrorTooShortPhone = "слишком короткий номер"

const MinPhoneLen = 10

func ParseRawTgText(text string) (phone int, err error) {
	var result strings.Builder
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if '0' <= ch && ch <= '9' {
			result.WriteByte(ch)
		}
	}
	phone, err = strconv.Atoi(result.String())
	if len(result.String()) < MinPhoneLen {
		return 0, errors.New(ErrorTooShortPhone)
	}
	return
}

func IncomingRFPhoneProcessing(num string) (result *ParsedResult, err error) {
	tmp := strings.Replace(num, "+", ``, -1)
	num = strings.Replace(tmp, " ", ``, -1)

	if len(num) == 10 {
		num = "7" + num
	}
	if len(num) < 11 {
		return nil, errors.New("num too short")
	}
	tmp = num[1:4]
	var (
		code  int
		phone int
	)
	code, err = strconv.Atoi(tmp)
	if err != nil {
		return nil, err
	}
	tmp = num[4:]
	phone, err = strconv.Atoi(tmp)
	return &ParsedResult{
		Code:    code,
		FullNum: num,
		Phone:   phone,
	}, nil
}
