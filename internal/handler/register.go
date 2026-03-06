package handler

import (
	_ "github.com/external-secrets/reloader/internal/handler/deployment"
	_ "github.com/external-secrets/reloader/internal/handler/externalsecret"
	_ "github.com/external-secrets/reloader/internal/handler/pushsecret"
	_ "github.com/external-secrets/reloader/internal/handler/workflow"
)
