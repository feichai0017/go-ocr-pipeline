package handlers

import (
	"github.com/feichai0017/document-processor/internal/service/document"
	"github.com/feichai0017/document-processor/pkg/logger"
)

type Handlers struct {
	Document *DocumentHandler
}

func NewHandlers(
	documentService document.DocumentProcessor,
	logger logger.Logger,
) *Handlers {
	return &Handlers{
		Document: NewDocumentHandler(documentService, logger),
	}
}
