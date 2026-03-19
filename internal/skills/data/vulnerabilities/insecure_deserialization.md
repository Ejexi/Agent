---
name: insecure-deserialization
description: Insecure deserialization — pickle, YAML, Java ObjectInputStream, PHP unserialize, gadget chains
---

# Insecure Deserialization

Deserializing untrusted data can lead to RCE, auth bypass, or DoS. The severity is critical when the deserializer executes code during the deserialization process itself.

## Critical Patterns by Language

### Python — pickle / yaml

```python
# ❌ CRITICAL: pickle executes arbitrary code on load
import pickle
data = pickle.loads(user_input)          # RCE
pickle.load(request.body)               # RCE

# ❌ CRITICAL: yaml.load with Loader=None uses unsafe loader
import yaml
yaml.load(user_input)                   # RCE in PyYAML < 6.0
yaml.load(data, Loader=yaml.Loader)     # RCE

# ✅ Safe alternatives
import json
json.loads(user_input)                  # safe — no code execution

yaml.safe_load(user_input)              # safe — no Python objects
yaml.load(data, Loader=yaml.SafeLoader) # explicit safe loader
```

**Pickle exploit:**
```python
import pickle, os

class Exploit(object):
    def __reduce__(self):
        return (os.system, ('id > /tmp/pwned',))

payload = pickle.dumps(Exploit())
# Sending payload to any pickle.loads() → RCE
```

### Java — ObjectInputStream

```java
// ❌ Deserializes any class on the classpath — gadget chains → RCE
ObjectInputStream ois = new ObjectInputStream(inputStream);
Object obj = ois.readObject();   // CRITICAL if input untrusted

// ❌ Common vulnerable frameworks
XStream xstream = new XStream();
xstream.fromXML(userInput);      // RCE via gadget chains

// ✅ Use typed deserialization with allowlisting
ObjectInputStream safe = new ValidatingObjectInputStream(inputStream);
safe.accept(MyExpectedClass.class);   // allowlist
```

### PHP — unserialize

```php
// ❌ __wakeup() / __destruct() magic methods → RCE, SSRF, SQLi
$obj = unserialize($_POST['data']);

// Common gadget chain target: POP chains in frameworks
// Laravel, Symfony, Zend all have known gadgets

// ✅ Use JSON instead
$data = json_decode($_POST['data'], true);
```

### Go — gob / encoding

```go
// gob is safer than pickle but still deserializes concrete types
// ❌ Deserializing interface{} allows type confusion
var v interface{}
gob.NewDecoder(r).Decode(&v)   // type confusion possible

// ✅ Decode into concrete known types only
var msg MyKnownStruct
gob.NewDecoder(r).Decode(&msg)

// JSON is safest — no code execution possible
json.NewDecoder(r).Decode(&msg)
```

## YAML Deserialization — Beyond Python

```yaml
# !!python/object/apply: exploits Python-specific YAML tags
!!python/object/apply:os.system ['id']

# !!java.lang.ProcessBuilder (snakeyaml)
!!java.lang.ProcessBuilder [["id"]]
```

## Detection Signals

- User-controlled data passed to: `pickle.loads`, `yaml.load`, `unserialize`, `readObject`, `XStream.fromXML`
- Base64-encoded blobs in cookies/headers (often serialized Java: starts with `rO0AB`)
- `VIEWSTATE` in ASP.NET without `machineKey` validation
- `__wakeup`, `__destruct`, `__toString` in PHP classes that handle resources

## Java Gadget Chains

Common libraries with known gadget chains (ysoserial):
- Apache Commons Collections
- Spring Framework
- Groovy
- JBoss Marshalling

```bash
# Generate payload with ysoserial
java -jar ysoserial.jar CommonsCollections6 "id" | base64
```

## Remediation

1. **Never deserialize untrusted data** — use JSON/protobuf instead
2. For Java: use `ObjectInputFilter` (JDK 9+) to allowlist classes
3. For Python: replace pickle with `json`, `msgpack`, or `protobuf`
4. For PHP: use `json_decode` — never `unserialize` with untrusted input
5. If deserialization is required: sign+verify the serialized data before deserializing
6. Run deserialization in a sandboxed process with minimal capabilities
