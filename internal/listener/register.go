package listener

import (
	_ "github.com/external-secrets/reloader/internal/listener/eventgrid"
	_ "github.com/external-secrets/reloader/internal/listener/hashivault"
	_ "github.com/external-secrets/reloader/internal/listener/k8sconfigmap"
	_ "github.com/external-secrets/reloader/internal/listener/k8ssecret"
	_ "github.com/external-secrets/reloader/internal/listener/mock"
	_ "github.com/external-secrets/reloader/internal/listener/pubsub"
	_ "github.com/external-secrets/reloader/internal/listener/sqs"
	_ "github.com/external-secrets/reloader/internal/listener/tcp"
)
