local s = "Olá, mundo! 123 abc 456"
print(s:find("mundo"), s:find("%d+"), s:find("(%d)(%d)"), s:find(".", 1, true), s:find("x"))
print(s:match("%d+"), s:match("(%a+) (%d+)"), s:match("()mundo()"))
for w in s:gmatch("%a+") do io.write(w, ";") end print()
for k, v in ("a=1, b=2"):gmatch("(%w+)=(%w+)") do io.write(k, "->", v, " ") end print()
print(s:gsub("%d", "#"))
print(("hello world"):gsub("o", "0", 1))
print(("hello world"):gsub("(%w+)", "<%1>"))
print(("hello"):gsub("", "-"))
print(("$name is $age"):gsub("%$(%w+)", {name = "Ana", age = 30}))
print(("abc"):gsub("%w", function(c) return c:upper() .. "." end))
print(("abc"):gsub("b", function() return nil end))
print(string.rep("ab", 3, "-"), ("abc"):reverse(), ("ABC"):lower(), ("abc"):byte(1, -1))
print(string.char(76, 117, 97), ("hello"):sub(2, 3), ("hello"):sub(-3), ("hello"):sub(2), ("hello"):sub(0), ("hello"):sub(10))
print(("  trim  "):match("^%s*(.-)%s*$") .. "|", ("key = value"):match("^(%w+)%s*=%s*(%w+)$"))
print(("THE (quick) fox"):find("%((%a+)%)"), ("f(a(b)c)d"):match("%b()"))
print(("hello world from lua"):gsub("%f[%a]%a", string.upper))
print(("x = 1, y = 2"):gsub("(%w+) = (%w+)", "%2 = %1"))
print(("abc"):find("b", -1), ("abc"):find("b", -2), ("aaa"):match("a-b"), ("aaab"):match("a-b"), ("ab"):match("a?b"))
print(("[test]"):match("%[(.*)%]"), ("a.b.c"):gsub("%.", "/"), ("1,2,,3"):gsub(",,", ",nil,"))
print(("café"):len(), utf8.len("café"), utf8.char(72, 228, 8364), utf8.codepoint("ä"))
for p, c in utf8.codes("aé") do io.write(p, ":", c, " ") end print()
print(utf8.offset("aéb", 3), utf8.charpattern == "[\0-\x7F\xC2-\xFD][\x80-\xBF]*")
print(tostring(nil), tostring(true), tostring(12), tostring(1.5), ("x"):rep(0), ("%"):rep(2))
print(#"\65\066\x43\u{48}\z
       done", "a\
b", [[long
string]], [==[with ]] inside]==])
print(("one two  three"):gsub("%s+", " "), select("#", ("a b"):gsub(" ", "")))
local parts = {}
for part in ("a,b,,c"):gmatch("([^,]*)") do parts[#parts + 1] = "<" .. part .. ">" end
print(table.concat(parts))
print(("hello"):find("l"), ("hello"):find("xl"), ("hello"):match(".-(l+)(.*)"))
print(string.format("%-10s|%10s|", "esq", "dir"), ("%5.1f"):format(3.14159))
