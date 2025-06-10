package paginate

import "strconv"

func Int64CursorParser() CursorParserFunc[int64] {
	return func(rawCursor string) (*int64, error) {
		num, err := strconv.ParseInt(rawCursor, 10, 64)
		if err != nil {
			return nil, err
		}
		return &num, nil
	}
}
