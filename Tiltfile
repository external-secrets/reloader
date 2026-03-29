# -*- mode: Python -*-

kubectl_cmd = "kubectl"

# verify kubectl command exists
if str(local("command -v " + kubectl_cmd + " || true", quiet = True)) == "":
    fail("Required command '" + kubectl_cmd + "' not found in PATH")

# set defaults
settings = {
    "debug": {
        "enabled": False,
    },
}

# merge default settings with user defined settings
tilt_file = "./tilt-settings.yaml" if os.path.exists("./tilt-settings.yaml") else "./tilt-settings.json"
settings.update(read_yaml(
    tilt_file,
    default = {},
))

# set up the development environment
local(kubectl_cmd + ' create namespace external-secrets-reloader --dry-run=client -o yaml | ' + kubectl_cmd + ' apply -f -', quiet = True)

yaml = helm(
    './deploy/charts/reloader',
    name = 'reloader',
    namespace = 'external-secrets-reloader',
    set = [
        'image.repository=ghcr.io/external-secrets/reloader',
        'image.tag=latest',
        'installCRDs=true',
        'securityContext.enabled=false',
        'podSecurityContext.enabled=false',
    ],
)

objects = decode_yaml_stream(yaml)
for o in objects:
    if o.get('kind') == 'Deployment' and o.get('metadata').get('name') == 'reloader-reloader':
        o['spec']['template']['spec']['containers'][0]['securityContext'] = {'runAsNonRoot': False, 'readOnlyRootFilesystem': False}
        o['spec']['template']['spec']['containers'][0]['imagePullPolicy'] = 'Always'
        if settings.get('debug').get('enabled'):
            o['spec']['template']['spec']['containers'][0]['ports'] = [{'containerPort': 30000}]

updated_install = encode_yaml_stream(objects)

k8s_yaml(updated_install, allow_duplicates = True)

load('ext://restart_process', 'docker_build_with_restart')

gcflags = ''
if settings.get('debug').get('enabled'):
    gcflags = '-N -l'

local_resource(
    'reloader-binary',
    "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags '{gcflags}' -v -o bin/reloader cmd/main.go".format(gcflags=gcflags),
    deps = [
        "go.mod",
        "go.sum",
        "api",
        "cmd",
        "pkg",
        "internal",
    ],
)

entrypoint = ['/reloader']
dockerfile = 'tilt.dockerfile'
if settings.get('debug').get('enabled'):
    k8s_resource('reloader', port_forwards=[
        port_forward(30000, 30000, 'debugger'),
    ])
    entrypoint = ['/dlv', '--listen=:30000', '--api-version=2', '--continue=true', '--accept-multiclient=true', '--headless=true', 'exec', '/reloader', '--']
    dockerfile = 'tilt.debug.dockerfile'


docker_build_with_restart(
    'ghcr.io/external-secrets/reloader',
    '.',
    dockerfile = dockerfile,
    entrypoint = entrypoint,
    only=[
      './bin',
    ],
    live_update = [
        sync('./bin/reloader', '/reloader'),
    ],
)
