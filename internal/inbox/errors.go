package inbox

import "fmt"

// ErrPreviewMessageInvalidType is returned when the preview message from database is not a string type
type ErrPreviewMessageInvalidType struct {
	PreviewMessage interface{}
}

func (e ErrPreviewMessageInvalidType) Error() string {
	return fmt.Sprintf("preview message is not a string, got type %T with value %v", e.PreviewMessage, e.PreviewMessage)
}

// ErrPreviewMessageNil is returned when the preview message from database is nil
type ErrPreviewMessageNil struct{}

func (e ErrPreviewMessageNil) Error() string {
	return "preview message is nil"
}

// ErrUnsupportedContentType is returned when the content type is not supported
type ErrUnsupportedContentType struct {
	ContentType string
}

func (e ErrUnsupportedContentType) Error() string {
	return fmt.Sprintf("content type %s not supported", e.ContentType)
}
