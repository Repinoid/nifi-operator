# Hardcoded Values

## NiFi Operator

### Passwords
```
keystorePassword = "parole"
truststorePassword = "parole"
keyPassword = "parole"
```

### Ports
```
HTTPS = 8443
SiteToSite = 10443
Metrics = 9092
JMX_RMI = 9010
```

### JMX Configuration
```
jmxremote.port = 9010
jmxremote.rmi.port = 9010
jmxremote.authenticate = false
jmxremote.ssl = false
rmi.server.hostname = "127.0.0.1"
```

### Security Context
```
runAsUser = 1000
runAsGroup = 1000
fsGroup = 1000
```

### Init Container
```
image = "busybox:latest"
```

### Resource Limits (defaults)
```
cpu.request = "500m"
cpu.limit = "2000m"
memory.request = "2Gi"
memory.limit = "4Gi"
jvm.heap = "2g"
```

### JVM Configuration
```
java.arg.2 = "-Djava.awt.headless=true"
java.arg.3 = "-XX:+UseG1GC"
java.arg.jmx.1 = "-Dcom.sun.management.jmxremote"
```

### Volumes & Paths
```
config.path = "/opt/nifi/nifi-current/conf"
nifi.home = "/opt/nifi/nifi-current"
logs.path = "/opt/nifi/nifi-current/logs"
flowfile.repo = "/opt/nifi/nifi-current/flowfile_repository"
content.repo = "/opt/nifi/nifi-current/content_repository"
provenance.repo = "/opt/nifi/nifi-current/provenance_repository"
database.repo = "/opt/nifi/nifi-current/database_repository"
state.dir = "/opt/nifi/nifi-current/state"
```

### Storage Defaults
```
content.storage = "50Gi"
database.storage = "5Gi"
flowfile.storage = "10Gi"
logs.storage = "5Gi"
provenance.storage = "10Gi"
state.storage = "1Gi"
```

### Network
```
cluster.protocol = "RAW"
cluster.is.secure = "true"
load.balance.port = "6342"
```

### Labels & Selectors
```
app.label = "nifi"
component.label = "nifi"
```

---

## NiFi Registry Operator

### Passwords
```
keystorePassword = "parole"
truststorePassword = "parole"
keyPassword = "parole"
```

### Ports
```
HTTPS = 18443
HTTP = 18080
```

### Security Context
```
runAsUser = 1000
runAsGroup = 1000
fsGroup = 1000
```

### Init Container
```
image = "busybox:latest"
```

### Resource Limits (defaults)
```
cpu.request = "500m"
cpu.limit = "1000m"
memory.request = "512Mi"
memory.limit = "1Gi"
```

### Volumes & Paths
```
config.path = "/opt/nifi-registry/nifi-registry-current/conf"
registry.home = "/opt/nifi-registry/nifi-registry-current"
database.dir = "/opt/nifi-registry/nifi-registry-current/database"
flow.storage.dir = "/opt/nifi-registry/nifi-registry-current/flow_storage"
extension.dir = "/opt/nifi-registry/nifi-registry-current/extension_bundles"
work.dir = "/opt/nifi-registry/nifi-registry-current/work/jetty"
```

### Storage Defaults
```
data.storage = "5Gi"
```

### PostgreSQL (embedded config)
```
postgres.image = "postgres:15-alpine"
postgres.port = 5432
postgres.user = "registryuser"
postgres.password = "registrypass"
postgres.database = "nifiregistry"
postgres.storage = "5Gi"
postgres.cpu.request = "250m"
postgres.cpu.limit = "500m"
postgres.memory.request = "256Mi"
postgres.memory.limit = "512Mi"
```

### Git Provider
```
git.remote.url = (required from CRD)
git.remote.user = (optional from CRD)
git.remote.password = (optional from CRD)
```

### Labels & Selectors
```
app.label = "nifi-registry"
component.label = "nifi-registry"
postgres.app.label = "registry-postgres"
```
