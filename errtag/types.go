package errtag

type Internal struct{ ErrorTag[codeInternal] }

type Unauthorized struct{ ErrorTag[codeUnauthorized] }

type InvalidArgument struct{ ErrorTag[codeBadRequest] }

type NotFound struct{ ErrorTag[codeNotFound] }

type Conflict struct{ ErrorTag[codeConflict] }

type Forbidden struct{ ErrorTag[forbidden] }
