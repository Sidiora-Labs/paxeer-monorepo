"""Offline proofs of the maintained Java source, without a subprocess toolchain.

A deliberately small Java statement translator executes the selected production
method bodies. Unsupported syntax fails instead of substituting an algorithm.
The host adapters provide strings, collections, ZIP, JSON and real POSIX files;
there is no second delivery-store or scanner implementation. These tests do not
certify JVM compilation, Android API support or power-loss behavior.
"""

import ast
import base64
from contextlib import contextmanager
import fcntl
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import sys
import tempfile
import threading
import tomllib
from types import SimpleNamespace
import zipfile

import pytest


ROOT = Path(__file__).resolve().parents[3]
JAVA = ROOT / "sdk/integrations/android/src/main/java/com/sidiora/layerx/android"
RECORD = Path(__file__).with_name("repair_22_33_614.kvx")
TOKEN = re.compile(
    r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\''
    r'|(?:0x[0-9a-fA-F]+|\d[\d_]*(?:\.\d+)?)[dDfFlL]?'
    r'|[A-Za-z_$][\w$]*|>>>|>>|<<|->|::|\+\+|--|&&|\|\||==|!=|<=|>=|[+*/%&|^-]=|[^\s]',
    re.S,
)


def source(name):
    return (JAVA / name).read_text(encoding="utf-8")


def tokens(text):
    return [m.group() for m in TOKEN.finditer(text) if not m.group().startswith(("//", "/*"))]


def body(text, signature):
    start = text.index(signature)
    opening = text.index("{", start)
    depth = 0
    for match in TOKEN.finditer(text, opening):
        token = match.group()
        if token == "{":
            depth += 1
        elif token == "}":
            depth -= 1
            if depth == 0:
                return text[opening + 1:match.start()]
    raise AssertionError(f"Unclosed Java method: {signature}")


def compact(text):
    return " ".join(tokens(text))


def require(text, *facts):
    for fact in facts:
        assert compact(fact) in compact(text), fact


def split_top(items, delimiter):
    result, current, stack = [], [], []
    for token in items:
        if token == delimiter and not stack:
            result.append(current)
            current = []
            continue
        current.append(token)
        if token in ("(", "[", "{"):
            stack.append(token)
        elif token in (")", "]", "}"):
            stack.pop()
    result.append(current)
    return result


def expression(items):
    """Translate Java expressions, preserving calls and short-circuit predicates."""
    if "?" in items:
        depth = 0
        question = None
        for index, token in enumerate(items):
            if token in ("(", "["):
                depth += 1
            elif token in (")", "]"):
                depth -= 1
            elif token == "?" and depth == 0:
                question = index
                break
        if question is not None:
            depth = 0
            nested = 0
            for colon in range(question + 1, len(items)):
                token = items[colon]
                if token in ("(", "["):
                    depth += 1
                elif token in (")", "]"):
                    depth -= 1
                elif depth == 0 and token == "?":
                    nested += 1
                elif depth == 0 and token == ":":
                    if nested == 0:
                        return ("(" + expression(items[question + 1:colon])
                                + " if (" + expression(items[:question]) + ") else "
                                + expression(items[colon + 1:]) + ")")
                    nested -= 1
            raise AssertionError("Conditional expression has no matching colon")
        # Translate the containing argument group before the surrounding call.
        for opening, token in enumerate(items):
            if token not in ("(", "["):
                continue
            depth = 1
            closing = opening + 1
            while depth:
                if items[closing] in ("(", "["):
                    depth += 1
                elif items[closing] in (")", "]"):
                    depth -= 1
                closing += 1
            if "?" in items[opening + 1:closing - 1]:
                nested_expression = ("(" + expression(items[opening + 1:closing - 1]) + ")")
                return expression(items[:opening] + [nested_expression] + items[closing:])
        raise AssertionError("Unsupported conditional-expression location")
    output, index = [], 0
    while index < len(items):
        token = items[index]
        if token == "new":
            index += 1
            name = []
            while index < len(items) and items[index] not in ("<", "(", "["):
                name.append(items[index])
                index += 1
            if index < len(items) and items[index] == "<":
                depth = 1
                index += 1
                while depth:
                    if items[index] == "<":
                        depth += 1
                    elif items[index] == ">":
                        depth -= 1
                    index += 1
            if name == ["TypeReference"]:
                assert items[index:index + 4] == ["(", ")", "{", "}"]
                output.append("None")
                index += 4
            elif items[index] == "[":
                closing = items.index("]", index)
                assert name == ["int"]
                output.append("JArray([0] * (" + expression(items[index + 1:closing]) + "))")
                index = closing + 1
            else:
                output.append("".join(name))
            continue
        if token == "->":
            parameter = output.pop()
            output.extend(["lambda", parameter, ":"])
        elif token == "++":
            name = items[index + 1]
            output.append(f"({name} := {name} + 1)")
            index += 1
        elif token.startswith('"'):
            output.append("JString(" + repr(json.loads(token)) + ")")
        elif token.startswith("'"):
            output.append("JChar(" + str(ord(ast.literal_eval(token))) + ")")
        elif re.fullmatch(r'(?:0x[0-9a-fA-F]+|\d[\d_]*(?:\.\d+)?)[dDfFlL]?', token):
            output.append(re.sub(r'[dDfFlL]$', '', token) if not token.startswith("0x") else token)
        else:
            output.append({
                "null": "None", "true": "True", "false": "False", "!": "not",
                "&&": "and", "||": "or", "=": ":=", "::": ".", "this": "receiver",
            }.get(token, token))
        index += 1
    return " ".join(output).replace("mutation . apply (", "mutation (")


class Statements:
    """Parse the actual method's blocks; no method-specific branch replacement."""

    def __init__(self, text):
        # The byte-to-character cast is the only cast in the selected bodies.
        self.items = tokens(text)
        self.pos = 0

    def take(self, expected=None):
        item = self.items[self.pos]
        self.pos += 1
        if expected is not None:
            assert item == expected, (expected, item)
        return item

    def peek(self):
        return self.items[self.pos] if self.pos < len(self.items) else None

    def balanced(self):
        self.take("(")
        start, depth = self.pos, 1
        while depth:
            token = self.take()
            depth += (token == "(") - (token == ")")
        return self.items[start:self.pos - 1]

    @staticmethod
    def indent(lines):
        return ["    " + line for line in (lines or ["pass"])]

    def block(self):
        if self.peek() != "{":
            return self.statement()
        self.take("{")
        lines = []
        while self.peek() != "}":
            lines.extend(self.statement())
        self.take("}")
        return lines

    @staticmethod
    def assignment(items):
        if "=" in items:
            index = items.index("=")
            left = items[:index]
            # Java local declarations end in their variable name.
            if len(left) > 1 and (left[0][0].isupper() or left[0] in ("int", "long", "double", "char", "byte", "boolean")):
                left = [left[-1]]
            right = items[index + 1:]
            if right[:3] == ["(", "char", ")"]:
                return expression(left) + " = chr(" + expression(right[3:]) + ")"
            return expression(left) + " = " + expression(right)
        if items[-1:] in (["++"], ["--"]):
            return expression(items[:-1]) + (" += 1" if items[-1] == "++" else " -= 1")
        if (len(items) >= 2 and "(" not in items and re.fullmatch(r"[A-Za-z_]\w*", items[-1])
                and (items[0][0].isupper() or items[0] in ("int", "long", "double", "char"))):
            return items[-1] + " = None"
        # compound assignments retain their Java/Python spelling.
        return expression(items)

    def statement(self):
        kind = self.peek()
        if kind == "if":
            self.take()
            condition = expression(self.balanced())
            lines = ["if " + condition + ":"] + self.indent(self.block())
            if self.peek() == "else":
                self.take()
                lines += ["else:"] + self.indent(self.block())
            return lines
        if kind in ("while", "for"):
            self.take()
            header = self.balanced()
            if kind == "while":
                return ["while " + expression(header) + ":"] + self.indent(self.block())
            parts = split_top(header, ";")
            if len(parts) == 3:
                return [self.assignment(parts[0]), "while " + expression(parts[1]) + ":"] + self.indent(
                    self.block() + [self.assignment(parts[2])]
                )
            colon = header.index(":")
            return ["for " + header[colon - 1] + " in " + expression(header[colon + 1:]) + ":"] + self.indent(self.block())
        if kind == "try":
            self.take()
            resources = self.balanced() if self.peek() == "(" else []
            lines = self.block()
            if resources:
                declarations = split_top(resources, ";")
                bindings = []
                for declaration in declarations:
                    equals = declaration.index("=")
                    bindings.append(expression(declaration[equals + 1:]) + " as " + declaration[equals - 1])
                lines = ["with " + ", ".join(bindings) + ":"] + self.indent(lines)
            if self.peek() not in ("catch", "finally"):
                return lines
            lines = ["try:"] + self.indent(lines)
            while self.peek() == "catch":
                self.take()
                caught = self.balanced()
                types = [expression(part) for part in split_top(caught[:-1], "|")]
                lines += ["except (" + ", ".join(types) + ",) as " + caught[-1] + ":"] + self.indent(self.block())
            if self.peek() == "finally":
                self.take()
                lines += ["finally:"] + self.indent(self.block())
            return lines
        start, stack = self.pos, []
        while True:
            token = self.take()
            if token == ";" and not stack:
                break
            if token in ("(", "[", "{"):
                stack.append(token)
            elif token in (")", "]", "}"):
                stack.pop()
        items = self.items[start:self.pos - 1]
        # A store mutation is a real production lambda, not a test-side state machine.
        if "->" in items and "{" in items:
            arrow = items.index("->")
            assert items[arrow + 1] == "{" and items[-2:] == ["}", ")"]
            nested = Statements(" ".join(items[arrow + 2:-2]))
            mutation = nested.all()
            call = items[:arrow - 1]
            assert call[-2:] == ["mutate", "("]
            returns = call[0] == "return"
            return ["def mutation(entries):"] + self.indent(mutation) + [
                ("return " if returns else "") + "mutate(mutation)"
            ]
        if items[:1] == ["return"]:
            return ["return " + expression(items[1:])]
        if items[:1] == ["throw"]:
            return ["raise " + expression(items[1:])]
        return [self.assignment(items)]

    def all(self):
        lines = []
        while self.peek() is not None:
            lines.extend(self.statement())
        return lines


class JChar(int):
    def __str__(self):
        return chr(self)


class JString(str):
    def __add__(self, other):
        return JString(str(self) + str(other))

    def equals(self, other):
        return self == other

    def length(self):
        return len(self)

    def isEmpty(self):
        return not self

    def charAt(self, index):
        return ord(self[index])

    def getBytes(self, encoding):
        return JBytes(self.encode(encoding))

    def indexOf(self, value):
        return self.find(chr(value) if isinstance(value, int) else value)

    def lastIndexOf(self, value):
        return self.rfind(chr(value) if isinstance(value, int) else value)

    def contains(self, value):
        return value in self

    def startsWith(self, value):
        return self.startswith(value)

    def endsWith(self, value):
        return self.endswith(value)

    def substring(self, start, end=None):
        return JString(self[start:end])

    def toLowerCase(self, locale):
        return JString(self.lower())

    def toString(self):
        return self

    def split(self, pattern, limit):
        assert limit == -1
        return JArray(JString(value) for value in re.split(pattern, self))

    def matches(self, pattern):
        return re.fullmatch(pattern, self) is not None


class JArray(list):
    @property
    def length(self):
        return len(self)


class JBytes(bytes):
    @property
    def length(self):
        return len(self)


class JList(list):
    def add(self, value):
        self.append(value)

    def addAll(self, values):
        self.extend(values)

    def isEmpty(self):
        return not self


class JSet(set):
    def contains(self, value):
        return value in self

    @staticmethod
    def of(*values):
        return JSet(values)


class JIterator:
    def __init__(self, mapping):
        self.mapping = mapping
        self.keys = iter(list(dict.keys(mapping)))
        self.pending = None
        self.current = None

    def hasNext(self):
        if self.pending is None:
            self.pending = next(self.keys, None)
        return self.pending is not None

    def next(self):
        assert self.hasNext()
        self.current, self.pending = self.pending, None
        return SimpleNamespace(getValue=lambda: self.mapping[self.current], getKey=lambda: self.current)

    def remove(self):
        del self.mapping[self.current]


class JMap(dict):
    def put(self, key, value):
        self[key] = value

    def remove(self, key):
        return self.pop(key, None)

    def size(self):
        return len(self)

    def entrySet(self):
        return self

    def __iter__(self):
        return iter([
            SimpleNamespace(getKey=lambda k=key: k, getValue=lambda v=value: v)
            for key, value in self.items()
        ])

    def iterator(self):
        return JIterator(self)

    def removeIf(self, predicate):
        for item in list(self):
            if predicate(item):
                del self[item.getKey()]


class JBuilder:
    def __init__(self, value=""):
        self.value = str(value)

    def append(self, value):
        self.value += str(value)
        return self

    def length(self):
        return len(self.value)

    def isEmpty(self):
        return not self.value

    def setLength(self, length):
        self.value = self.value[:length]

    def substring(self, start):
        return JString(self.value[start:])

    def toString(self):
        return JString(self.value)


class JPattern:
    def __init__(self, pattern):
        self.pattern = re.compile(pattern)

    def matcher(self, value):
        matches = iter(self.pattern.finditer(value))
        state = SimpleNamespace(current=None)

        def find():
            state.current = next(matches, None)
            return state.current is not None

        return SimpleNamespace(find=find, group=lambda: JString(state.current.group()))


class JPath:
    def __init__(self, value):
        self.value = Path(value)

    def __str__(self):
        return str(self.value)

    def __add__(self, other):
        return JString(str(self) + str(other))

    def getParent(self):
        return JPath(self.value.parent)

    def getFileName(self):
        return JPath(self.value.name)

    def toString(self):
        return JString(str(self))

    def relativize(self, other):
        return JPath(other.value.relative_to(self.value))

    @staticmethod
    def of(value):
        return JPath(value)


class JInput:
    def __init__(self, handle):
        self.handle = handle

    def read(self):
        value = self.handle.read(1)
        return value[0] if value else -1

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.handle.close()


class JZip:
    def __init__(self, stream):
        self.stream = stream
        self.archive = zipfile.ZipFile(stream.handle)
        self.entries = iter(self.archive.infolist())
        self.current = None

    def getNextEntry(self):
        entry = next(self.entries, None)
        if entry is None:
            return None
        self.current = self.archive.open(entry)
        return SimpleNamespace(getName=lambda: JString(entry.filename), isDirectory=entry.is_dir)

    def closeEntry(self):
        self.current.close()
        self.current = None

    def read(self):
        value = self.current.read(1)
        return value[0] if value else -1

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.archive.close()
        self.stream.handle.close()


class JStream:
    def __init__(self, values):
        self.values = values

    def filter(self, predicate):
        return JStream([value for value in self.values if predicate(value)])

    def sorted(self):
        return JStream(sorted(self.values, key=str))

    def toList(self):
        return JList(self.values)

    def __enter__(self):
        return self

    def __exit__(self, *args):
        pass


class JBuffer:
    def __init__(self, data):
        self.data = data
        self.position = 0

    def hasRemaining(self):
        return self.position < len(self.data)

    @staticmethod
    def wrap(data):
        return JBuffer(data)


class JChannel:
    def __init__(self, path, *options):
        flags = os.O_RDONLY if "READ" in options else os.O_WRONLY
        if "CREATE" in options:
            flags |= os.O_CREAT
        if "TRUNCATE_EXISTING" in options:
            flags |= os.O_TRUNC
        self.fd = os.open(str(path), flags, 0o600)

    @staticmethod
    def open(path, *options):
        return JChannel(path, *options)

    def write(self, buffer):
        written = os.write(self.fd, buffer.data[buffer.position:])
        assert written > 0
        buffer.position += written

    def force(self, metadata):
        assert metadata is True
        os.fsync(self.fd)

    @contextmanager
    def lock(self):
        fcntl.flock(self.fd, fcntl.LOCK_EX)
        try:
            yield self
        finally:
            fcntl.flock(self.fd, fcntl.LOCK_UN)

    def __enter__(self):
        return self

    def __exit__(self, *args):
        os.close(self.fd)


class JFiles:
    exists = staticmethod(lambda path: path.value.exists())
    isDirectory = staticmethod(lambda path: path.value.is_dir())
    isRegularFile = staticmethod(lambda path: path.value.is_file())
    size = staticmethod(lambda path: path.value.stat().st_size)
    readAllBytes = staticmethod(lambda path: JBytes(path.value.read_bytes()))
    newInputStream = staticmethod(lambda path: JInput(path.value.open("rb")))
    walk = staticmethod(lambda path: JStream([JPath(item) for item in path.value.rglob("*")]))
    createDirectories = staticmethod(lambda path: path.value.mkdir(parents=True, exist_ok=True))
    setPosixFilePermissions = staticmethod(lambda path, mode: os.chmod(str(path), mode))

    @staticmethod
    def createTempFile(directory, prefix, suffix):
        fd, path = tempfile.mkstemp(dir=str(directory), prefix=prefix, suffix=suffix)
        os.close(fd)
        return JPath(path)

    @staticmethod
    def move(temporary, target, atomic, replace):
        assert (atomic, replace) == ("ATOMIC_MOVE", "REPLACE_EXISTING")
        os.replace(str(temporary), str(target))

    @staticmethod
    def deleteIfExists(path):
        path.value.unlink(missing_ok=True)


class JRecord:
    def __init__(self, **fields):
        self.fields = fields

    def __getattr__(self, name):
        return lambda: self.fields[name]


def plain(value):
    if isinstance(value, JRecord):
        return {key: plain(item) for key, item in value.fields.items()}
    if isinstance(value, dict):
        return {key: plain(item) for key, item in value.items()}
    return value


class JMapper:
    @staticmethod
    def writeValueAsBytes(value):
        return JBytes(json.dumps(plain(value), separators=(",", ":")).encode())

    @staticmethod
    def readValue(encoded, type_reference):
        assert type_reference is None
        data = json.loads(encoded)
        entries = data.get("entries")
        if entries is not None:
            entries = JMap({JString(key): JRecord(
                payloadDigest=JString(value["payloadDigest"]),
                leaseUntilMs=value["leaseUntilMs"], completed=value["completed"],
            ) for key, value in entries.items()})
        return JRecord(version=data["version"], entries=entries)


class JavaFailure(RuntimeError):
    def __init__(self, code):
        super().__init__(code)
        self.code = code

    @staticmethod
    def of(code):
        return JavaFailure(code)


JavaFailure.Code = SimpleNamespace(**{code: code for code in (
    "DELIVERY_STORE_FAILURE", "EVENT_REPLAY", "INVALID_CONFIGURATION",
)})


class AtomicMoveNotSupportedException(OSError):
    pass


class Program:
    """Execute source-derived functions with a fixed, explicit Java library bridge."""

    def __init__(self, text):
        self.text = text
        self.namespace = {
            "JString": JString, "JChar": JChar, "JArray": JArray, "StringBuilder": JBuilder,
            "ArrayList": JList, "LinkedHashMap": JMap, "Set": JSet,
            "StandardCharsets": SimpleNamespace(UTF_8="utf-8"),
            "Locale": SimpleNamespace(ROOT="ROOT"), "Files": JFiles, "Path": JPath,
            "FileChannel": JChannel, "ByteBuffer": JBuffer,
            "StandardOpenOption": SimpleNamespace(**{name: name for name in (
                "READ", "WRITE", "CREATE", "TRUNCATE_EXISTING",
            )}),
            "StandardCopyOption": SimpleNamespace(ATOMIC_MOVE="ATOMIC_MOVE", REPLACE_EXISTING="REPLACE_EXISTING"),
            "PosixFilePermissions": SimpleNamespace(fromString=lambda value: {"rwx------": 0o700, "rw-------": 0o600}[value]),
            "MobileIntegrationException": JavaFailure, "IOException": OSError,
            "RuntimeException": RuntimeError, "IllegalArgumentException": ValueError,
            "AtomicMoveNotSupportedException": AtomicMoveNotSupportedException,
            "ZipInputStream": JZip,
            "java": SimpleNamespace(io=SimpleNamespace(BufferedInputStream=lambda value: value)),
            "Math": SimpleNamespace(log=math.log),
            "Base64": SimpleNamespace(getUrlDecoder=lambda: SimpleNamespace(
                decode=lambda value: JBytes(base64.b64decode(value, altchars=b"-_", validate=True))
            )),
            "String": lambda value, encoding: JString(value.decode(encoding)),
        }
        self.lock = threading.RLock()
        self.translations = {}

    def add(self, signature, name, parameters, global_fields=()):
        method_body = body(self.text, signature)
        # Java's (char) value occurs only as StringBuilder.append's argument.
        method_body = method_body.replace("current.append((char) value)", "current.append(character(value))")
        self.namespace["character"] = chr
        lines = Statements(method_body).all()
        if global_fields:
            lines.insert(0, "global " + ", ".join(global_fields))
        generated = "def " + name + "(" + parameters + "):\n" + "\n".join(Statements.indent(lines)) + "\n"
        self.translations[name] = generated
        exec(compile(generated, str(JAVA / "<source-method>"), "exec"), self.namespace)

    def __getattr__(self, name):
        function = self.namespace[name]

        def call(*args):
            with self.lock:
                return function(*args)

        return call


def numeric_constants(text):
    result = {}
    for name, value in re.findall(r"private static final (?:int|long) (\w+) = ([^;]+);", text):
        tree = ast.parse(expression(tokens(value)), mode="eval")
        assert all(isinstance(node, (ast.Expression, ast.Constant, ast.BinOp, ast.Mult, ast.Add, ast.Sub, ast.Load)) for node in ast.walk(tree))
        result[name] = eval(compile(tree, "<java-constant>", "eval"), {"__builtins__": {}})
    return result


def scanner_program():
    text = source("EmbeddedSecretDetector.java")
    program = Program(text)
    prefixes_section = text.split("PROVIDER_PREFIXES =", 1)[1].split(";", 1)[0]
    prefixes = re.findall(r'Map.entry\("([^"]+)", "([^"]+)"\)', prefixes_section)
    assert len(prefixes) == 18
    regex_literals = re.search(
        r'Pattern.compile\(\s*("(?:\\.|[^"\\])*") \+ Pattern.quote\(prefix.getValue\(\)\)\s*\+ ("(?:\\.|[^"\\])*")', text
    )
    assert regex_literals is not None
    left, right = [json.loads(value) for value in regex_literals.groups()]
    patterns = JMap({JString(rule): JPattern(left + re.escape(prefix) + right) for rule, prefix in prefixes})
    markers_section = text.split("PEM_MARKERS =", 1)[1].split(";", 1)[0]
    markers = JMap({JString(key): JString(value) for key, value in re.findall(r'"([^"]+)"\s*,\s*"([^"]+)"', markers_section)})
    extensions_section = text.split("TEXTUAL_EXTENSIONS =", 1)[1].split(";", 1)[0]
    extension_names = JSet(re.findall(r'"([^"]+)"', extensions_section))
    token_pattern = re.search(r'TEXT_VALUE_TOKEN = Pattern.compile\(("[^"]+")\)', text)
    program.namespace.update(numeric_constants(text))
    program.namespace.update(PROVIDER_PATTERNS=patterns, PEM_MARKERS=markers, TEXTUAL_EXTENSIONS=extension_names,
                             TEXT_VALUE_TOKEN=JPattern(json.loads(token_pattern[1])))
    methods = [
        ("public static String providerCredentialRule(", "providerCredentialRule", "value"),
        ("public static boolean isSignedJsonWebToken(", "isSignedJsonWebToken", "value"),
        ("public static boolean isHighEntropyMaterial(", "isHighEntropyMaterial", "value"),
        ("public static double shannonEntropyBitsPerCharacter(", "shannonEntropyBitsPerCharacter", "bytes"),
        ("public static List<Finding> scan(", "scan", "stream, path, textual, exempt"),
        ("public static List<Finding> scanArtifact(", "scanArtifact", "root, exempt"),
        ("private static List<Finding> scanFile(", "scanFile", "file, path, exempt"),
        ("private static List<Finding> scanArchive(", "scanArchive", "file, exempt"),
        ("private static void appendRun(", "appendRun", "findings, current, runStart, path, textual, exempt"),
        ("private static String classifyRun(", "classifyRun", "value, textual, exempt"),
        ("private static boolean isTextual(Path", "isTextualPath", "file"),
        ("private static boolean isTextual(String", "isTextualName", "name"),
        ("private static boolean isArchive(", "isArchive", "file"),
        ("private static String padded(", "padded", "segment"),
        ("private static boolean isBase64Character(", "isBase64Character", "character"),
        ("private static boolean isBase64UrlCharacter(", "isBase64UrlCharacter", "character"),
        ("private static boolean isHexCharacter(", "isHexCharacter", "character"),
    ]
    for signature, name, parameters in methods:
        program.add(signature, name, parameters)
    program.namespace["isTextual"] = lambda value: program.isTextualPath(value) if isinstance(value, JPath) else program.isTextualName(value)
    finding_expression = tokens(body(text, "@Override public String toString()"))[1:-1]

    class Finding:
        def __init__(self, rule, path, offset, length):
            self.rule, self.path, self.offset, self.length = rule, path, offset, length

        def __str__(self):
            return eval(expression(finding_expression), {"JString": JString}, vars(self))

    program.namespace["Finding"] = Finding

    def bounded_input(stream, maximum):
        bounded = Program(text)
        bounded.namespace.update(source=stream, maximum=maximum, consumed=0)
        bounded.add("@Override public int read()", "read", "", global_fields=("consumed",))
        return bounded

    program.namespace["BoundedInputStream"] = bounded_input
    return program


def store_program(name, path=None, capacity=1, now=100):
    text = source(name + ".java")
    program = Program(text)
    program.namespace.update(capacity=capacity, clock=SimpleNamespace(getAsLong=lambda: now),
                             Claim=SimpleNamespace(**{value: value for value in ("CLAIMED", "COMPLETED", "PROCESSING", "CONFLICT")}))
    program.add("public " + ("synchronized " if name.startswith("InMemory") else "") + "Claim claim(", "claim", "deliveryId, payloadDigest, leaseUntilMs")
    for method in ("complete", "release"):
        program.add("public " + ("synchronized " if name.startswith("InMemory") else "") + "void " + method + "(", method, "deliveryId, payloadDigest")
    if name.startswith("InMemory"):
        program.namespace["entries"] = JMap()
        constructor = body(text, "private Entry(")
        code = "def Entry(payloadDigest, leaseUntilMs):\n    receiver = SimpleNamespace(completed=False)\n"
        code += "\n".join(Statements.indent(Statements(constructor).all())) + "\n    return receiver\n"
        program.namespace["SimpleNamespace"] = SimpleNamespace
        exec(compile(code, "<java-entry-constructor>", "exec"), program.namespace)
        program.add("private void evict(", "evict", "")
    else:
        program.namespace.update(numeric_constants(text))
        program.namespace.update(
            ledgerPath=JPath(path), lockPath=JPath(str(path) + ".lock"), mapper=JMapper(),
            Entry=lambda digest, lease, completed: JRecord(payloadDigest=digest, leaseUntilMs=lease, completed=completed),
            Ledger=lambda version, entries: JRecord(version=version, entries=entries),
        )
        for signature, method, parameters in (
            ("private <T> T mutate(", "mutate", "mutation"),
            ("private Map<String, Entry> read(", "read", ""),
            ("private void write(", "write", "entries"),
            ("private void evict(", "evict", "entries"),
            ("private static void setDirectoryPermissions(", "setDirectoryPermissions", "directory"),
            ("private static void setFilePermissions(", "setFilePermissions", "file"),
            ("private static void requireClaim(", "requireClaim", "deliveryId, payloadDigest, leaseUntilMs"),
            ("private static void requireIdentifier(", "requireIdentifier", "value"),
            ("private static void requireDigest(", "requireDigest", "value"),
            ("private static MobileIntegrationException failure(", "failure", ""),
        ):
            program.add(signature, method, parameters)
    return program


def test_010_packaged_structured_credentials_refused_without_value(tmp_path, capsys):
    scanner = scanner_program()
    cli = Program(source("EmbeddedSecretScan.java"))
    cli.namespace.update(EmbeddedSecretDetector=scanner)
    cli.namespace["System"] = SimpleNamespace(
        err=SimpleNamespace(println=lambda value: print(value, file=sys.stderr)),
        out=SimpleNamespace(println=print), exit=lambda code: sys.exit(code),
    )
    cli.add("public static void main(", "main", "arguments")
    values = {
        "assets/config.json": '{"provider":{"value":"sk-' + "a" * 24 + '"}}',
        "res/values/config.xml": '<resources><item value="ghp_' + "b" * 24 + '"/></resources>',
        "assets/config.properties": 'provider.value=AKIA' + "C" * 16 + '\n',
    }
    for suffix in ("apk", "aab", "jar", "zip"):
        artifact = tmp_path / ("application." + suffix)
        with zipfile.ZipFile(artifact, "w", zipfile.ZIP_DEFLATED) as archive:
            for name, payload in values.items():
                archive.writestr(name, payload)
            archive.writestr("AndroidManifest.xml", '<manifest package="com.example.clean"/>')
        with pytest.raises(SystemExit) as refused:
            cli.main(JArray([JString(str(artifact))]))
        assert refused.value.code == 1
        output = capsys.readouterr()
        assert not output.out
        for name, payload in values.items():
            assert name in output.err
            for secret in re.findall(r'(?:sk-|ghp_|AKIA)[A-Za-z0-9]+', payload):
                assert secret not in output.err
        assert output.err.count("rule=") == 3
        assert "length=" in output.err
    clean = tmp_path / "clean.apk"
    with zipfile.ZipFile(clean, "w") as archive:
        archive.writestr("assets/config.json", '{"provider":"publishable"}')
    with pytest.raises(SystemExit) as accepted:
        cli.main(JArray([JString(str(clean))]))
    assert accepted.value.code == 0
    assert "no embedded secret material" in capsys.readouterr().out


def test_010_provider_bounds_run_overlap_and_token_exemptions():
    scanner = scanner_program()
    rules = scanner.namespace["PROVIDER_PATTERNS"]
    for rule, pattern in rules.items():
        marker_section = source("EmbeddedSecretDetector.java").split("PROVIDER_PREFIXES =", 1)[1].split(";", 1)[0]
        prefix = re.search(r'Map.entry\("' + re.escape(rule) + r'", "([^"]+)"\)', marker_section)[1]
        credential = prefix + "a" * 24
        assert scanner.providerCredentialRule(JString('value="' + credential + '"')) == rule
        assert scanner.providerCredentialRule(JString(prefix + "a" * 11)) is None
        assert scanner.providerCredentialRule(JString("identifier" + credential)) is None
        assert scanner.providerCredentialRule(JString(prefix + "a" * 4097)) is None
        assert pattern.matcher(JString('"' + credential + '"')).find()
    for padding in (8180, 8190, 8192, 12280):
        raw = ('{"padding":"' + "a" * padding + '","value":"ghp_' + "b" * 24 + '"}').encode()
        findings = scanner.scan(JInput(io.BytesIO(raw)), JString("config.json"), True, JSet())
        assert findings and all(item.rule == "github-token" for item in findings)
        assert all(item.offset >= 0 and item.offset + item.length <= len(raw) for item in findings)
    public = "0x" + hashlib.sha256(b"publishable receipt key").hexdigest()
    raw = ('{"public_key":"' + public + '"}').encode()
    assert scanner.scan(JInput(io.BytesIO(raw)), JString("config.json"), True, JSet([public])) == []
    assert scanner.scan(JInput(io.BytesIO(raw)), JString("config.json"), True, JSet())
    jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJleGFtcGxlIn0." + "a" * 24
    result = scanner.scan(JInput(io.BytesIO(('{"value":"' + jwt + '"}').encode())), JString("config.json"), True, JSet())
    assert result[0].rule == "signed-json-web-token"


def test_009_directory_durability_precedes_acknowledgement():
    text = source("FileEventDeliveryStore.java")
    write = body(text, "private void write(")
    require(write, "output.force(true);", "StandardCopyOption.ATOMIC_MOVE", "StandardCopyOption.REPLACE_EXISTING",
            "try (FileChannel directory = FileChannel.open(ledgerPath.getParent(), StandardOpenOption.READ)) { directory.force(true); }")
    assert write.index("output.force(true)") < write.index("Files.move(") < write.index("directory.force(true)")
    mutate = body(text, "private <T> T mutate(")
    assert mutate.index("channel.lock()") < mutate.index("read()") < mutate.index("mutation.apply(entries)") < mutate.index("write(entries)") < mutate.index("return result")
    require(mutate, "catch (IOException | RuntimeException error) { throw failure(); }")
    consumer = body(source("VerifiedEventConsumer.java"), "public Outcome consume(")
    assert consumer.index("handler.handle(") < consumer.index("deliveries.complete(") < consumer.index("return Outcome.PROCESSED")
    require(consumer, "case COMPLETED -> { return Outcome.DUPLICATE; }", "case PROCESSING -> { return Outcome.PROCESSING; }")
    require(body(text, "private void evict("), "!item.getValue().completed() && item.getValue().leaseUntilMs() <= now")


def test_009_completed_claim_survives_abrupt_stop_and_reopen(tmp_path):
    ledger = tmp_path / "private" / "deliveries.json"
    digest = JString(hashlib.sha256(b"real delivery bytes").hexdigest())
    read_fd, write_fd = os.pipe()
    child = os.fork()
    if child == 0:
        os.close(read_fd)
        try:
            component = store_program("FileEventDeliveryStore", ledger)
            assert component.claim(JString("delivery-a"), digest, 1000) == "CLAIMED"
            component.complete(JString("delivery-a"), digest)
            os.write(write_fd, b"ack")
            # No close/destructor/shutdown path can flush state after this point.
            os._exit(0)
        except BaseException:
            import traceback
            traceback.print_exc()
            os._exit(1)
    os.close(write_fd)
    try:
        acknowledgement = os.read(read_fd, 3)
    finally:
        os.close(read_fd)
    _, status = os.waitpid(child, 0)
    assert os.waitstatus_to_exitcode(status) == 0
    assert acknowledgement == b"ack"
    reopened = store_program("FileEventDeliveryStore", ledger, now=100_000)
    assert reopened.claim(JString("delivery-a"), digest, 200_000) == "COMPLETED"
    assert reopened.claim(JString("delivery-a"), JString("f" * 64), 200_000) == "CONFLICT"
    recovered = json.loads(ledger.read_text())
    assert recovered["version"] == 1
    assert recovered["entries"]["delivery-a"] == {"payloadDigest": digest, "leaseUntilMs": 0, "completed": True}
    assert ledger.stat().st_mode & 0o777 == 0o600
    assert ledger.parent.stat().st_mode & 0o777 == 0o700
    assert not list(ledger.parent.glob("*.tmp"))


def test_041_capacity_one_overlap_preserves_live_claim_and_completed_retention():
    component = store_program("InMemoryEventDeliveryStore")
    first, second, digest = JString("a"), JString("b"), JString("d" * 64)
    running, finish = threading.Event(), threading.Event()
    effects, failures = [], []

    def deliver_first():
        try:
            assert component.claim(first, digest, 1000) == "CLAIMED"
            effects.append(first)
            running.set()
            assert finish.wait(5)
            component.complete(first, digest)
        except BaseException as error:
            failures.append(error)
            running.set()

    worker = threading.Thread(target=deliver_first)
    worker.start()
    try:
        assert running.wait(5)
        with pytest.raises(JavaFailure) as refused:
            component.claim(second, digest, 1000)
        assert refused.value.code == "DELIVERY_STORE_FAILURE"
        assert component.claim(first, digest, 1000) == "PROCESSING"
        assert effects == [first]
        assert len(component.namespace["entries"]) == 1
    finally:
        finish.set()
        worker.join(5)
    assert not worker.is_alive()
    assert failures == []
    assert component.claim(first, digest, 100_000) == "COMPLETED"
    component.release(first, digest)
    with pytest.raises(JavaFailure):
        component.claim(second, digest, 100_000)
    assert component.claim(first, JString("e" * 64), 100_000) == "CONFLICT"
    assert effects == [first]
    text = source("InMemoryEventDeliveryStore.java")
    assert "Completed claims are retained for this store's lifetime" in text
    for name in ("claim", "complete", "release"):
        assert re.search(r"public synchronized \w+ " + name + r"\(", text)
    consumer = body(source("VerifiedEventConsumer.java"), "public Outcome consume(")
    assert consumer.index("deliveries.claim(") < consumer.index("handler.handle(")
    require(consumer, "case PROCESSING -> { return Outcome.PROCESSING; }")


def test_041_only_expired_uncompleted_claims_free_capacity():
    digest = JString("a" * 64)
    component = store_program("InMemoryEventDeliveryStore", now=100)
    assert component.claim(JString("expired"), digest, 99) == "CLAIMED"
    assert component.claim(JString("new"), digest, 1000) == "CLAIMED"
    component.release(JString("new"), JString("b" * 64))
    assert component.claim(JString("new"), digest, 1000) == "PROCESSING"
    component.release(JString("new"), digest)
    assert component.claim(JString("replacement"), digest, 1000) == "CLAIMED"
    eviction = body(source("InMemoryEventDeliveryStore.java"), "private void evict(")
    require(eviction, "if (!entry.completed && entry.leaseUntilMs <= now) iterator.remove();")
    assert "oldest" not in eviction and "keySet" not in eviction


@pytest.mark.parametrize("host,expected", [
    ("localhost", True), ("LOCALHOST", True), ("::1", True), ("[::1]", True),
    ("127.0.0.1", True), ("127.1.2.3", True), ("127.255.255.255", True), ("127.0.0.0", True),
    ("127.attacker.example", False), ("127.0.0.1.attacker.example", False),
    ("localhost.attacker.example", False), ("127.256.0.1", False), ("127.0.0.999", False),
    ("127.0.0.-1", False), ("127.0.0.+1", False), ("127.0.0.1.", False),
    ("127.0.1", False), ("127.1", False), ("2130706433", False), ("0177.0.0.1", False),
    ("127.00.0.1", False), ("127..0.1", False), ("127.0.0.١", False),
    ("126.0.0.1", False), ("128.0.0.1", False), ("0.0.0.0", False), ("[::ffff:127.0.0.1]", False),
])
def test_007_numeric_loopback_literals_and_dns_lookalikes(host, expected):
    program = Program(source("PublishableConfiguration.java"))
    program.add("static boolean isLoopback(", "isLoopback", "host")
    assert program.isLoopback(JString(host)) is expected
    endpoint = body(program.text, "static URI endpoint(")
    require(endpoint, 'if (scheme.equals("https")) return uri;',
            'if (scheme.equals("http") && isLoopback(uri.getHost())) return uri;', "throw invalid();")
    assert "InetAddress" not in program.text


def test_record_maps_every_finding_to_executed_source_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert set(record["finding"]) == {"010", "009", "041", "007"}
    test_names = {node.name for node in ast.parse(Path(__file__).read_text()).body
                  if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    audit = (ROOT / "spec/layerx-merge/programme/audit/records/22.30.163.kvx").read_text()
    for identifier, finding in record["finding"].items():
        section = re.search(r"^\[finding\." + identifier + r"\]\n(.*?)(?=^\[|\Z)", audit, re.M | re.S)
        original = tomllib.loads(section[1])
        for key in ("file", "line", "class", "severity", "expected", "owner_domain"):
            assert finding[key] == original[key]
        assert finding["tests"] and set(finding["tests"]) <= test_names
    task = record["task"]
    assert task["id"] == "22.33.614"
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_614.py"
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "final hf-done JSON" in task["verified_revision"]
    assert "No pre-change verification" in task["baseline"]
    assert "JVM" in task["review_only"] and "power-loss" in task["review_only"]
    assert task["wiring_requests"] == []
    # The Python gate never shells out, skips or substitutes a separate algorithm.
    own = ast.parse(Path(__file__).read_text())
    assert not any(isinstance(node, (ast.Import, ast.ImportFrom)) and any(alias.name == "subprocess" for alias in node.names) for node in ast.walk(own))
    assert not any(isinstance(node, ast.Attribute) and node.attr in ("skip", "skipif", "xfail") for node in ast.walk(own))
