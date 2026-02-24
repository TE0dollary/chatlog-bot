package errors

import "net/http"

func InvalidArg(arg string) error {
	return Newf(nil, http.StatusBadRequest, "invalid argument: %s", arg)
}

