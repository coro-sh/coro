package errtag

type Unknown struct{ ErrorTag[codeInternal] }

type Unauthorized struct{ ErrorTag[codeUnauthorized] }

type InvalidArgument struct{ ErrorTag[codeBadRequest] }

type NotFound struct{ ErrorTag[codeNotFound] }

type Conflict struct{ ErrorTag[codeConflict] }
