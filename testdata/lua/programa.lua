-- um programa "de verdade": JSON, classes, fila de prioridade, memoização
local json = {}
local escapes = {['"'] = '\\"', ['\\'] = '\\\\', ['\n'] = '\\n', ['\t'] = '\\t'}
local function is_array(t)
  local n = 0
  for _ in pairs(t) do n = n + 1 end
  return n == #t
end
function json.encode(v)
  local tv = type(v)
  if tv == "nil" then return "null"
  elseif tv == "boolean" or tv == "number" then return tostring(v)
  elseif tv == "string" then return '"' .. v:gsub('[%c"\\]', escapes) .. '"'
  elseif tv == "table" then
    local out = {}
    if is_array(v) then
      for i = 1, #v do out[i] = json.encode(v[i]) end
      return "[" .. table.concat(out, ",") .. "]"
    end
    local keys = {}
    for k in pairs(v) do keys[#keys + 1] = k end
    table.sort(keys)
    for _, k in ipairs(keys) do out[#out + 1] = json.encode(tostring(k)) .. ":" .. json.encode(v[k]) end
    return "{" .. table.concat(out, ",") .. "}"
  end
  error("tipo não suportado: " .. tv)
end
function json.decode(s)
  local pos = 1
  local function ws() pos = s:find("%S", pos) or #s + 1 end
  local value
  local function str()
    local out = {}
    pos = pos + 1
    while true do
      local c = s:sub(pos, pos)
      if c == '"' then pos = pos + 1; break end
      if c == "\\" then
        local n = s:sub(pos + 1, pos + 1)
        out[#out + 1] = ({n = "\n", t = "\t"})[n] or n
        pos = pos + 2
      else
        out[#out + 1] = c
        pos = pos + 1
      end
    end
    return table.concat(out)
  end
  function value()
    ws()
    local c = s:sub(pos, pos)
    if c == "{" then
      local obj = {}
      pos = pos + 1; ws()
      if s:sub(pos, pos) == "}" then pos = pos + 1; return obj end
      repeat
        ws()
        local k = str(); ws()
        assert(s:sub(pos, pos) == ":"); pos = pos + 1
        obj[k] = value(); ws()
        local d = s:sub(pos, pos); pos = pos + 1
      until d == "}"
      return obj
    elseif c == "[" then
      local arr = {}
      pos = pos + 1; ws()
      if s:sub(pos, pos) == "]" then pos = pos + 1; return arr end
      repeat
        arr[#arr + 1] = value(); ws()
        local d = s:sub(pos, pos); pos = pos + 1
      until d == "]"
      return arr
    elseif c == '"' then return str()
    elseif s:find("^true", pos) then pos = pos + 4; return true
    elseif s:find("^false", pos) then pos = pos + 5; return false
    elseif s:find("^null", pos) then pos = pos + 4; return nil
    else
      local num = s:match("^-?%d+%.?%d*[eE]?[-+]?%d*", pos)
      pos = pos + #num
      return math.tointeger(tonumber(num)) or tonumber(num)
    end
  end
  return value()
end
local data = {nome = "Ed.Tour", pacotes = {"Chapada", "Morro", "Lençóis"}, preco = 500, ativo = true, nota = 4.5, texto = 'linha "1"\nlinha 2'}
local enc = json.encode(data)
print(enc)
local dec = json.decode(enc)
print(dec.nome, #dec.pacotes, dec.pacotes[3], dec.preco, math.type(dec.preco), dec.ativo, dec.nota, dec.texto == data.texto)
print(json.encode(json.decode('{"a":[1,2,{"b":null}],"c":"x\\ty"}')))

-- classes com herança e super
local function class(parent)
  local c = {}
  c.__index = c
  if parent then setmetatable(c, {__index = parent}) end
  c.new = function(...) local o = setmetatable({}, c); if o.init then o:init(...) end; return o end
  return c
end
local Shape = class()
function Shape:init(name) self.name = name end
function Shape:describe() return self.name .. " com área " .. string.format("%.2f", self:area()) end
local Circle = class(Shape)
function Circle:init(r) Shape.init(self, "círculo"); self.r = r end
function Circle:area() return math.pi * self.r ^ 2 end
local Rect = class(Shape)
function Rect:init(w, h) Shape.init(self, "retângulo"); self.w, self.h = w, h end
function Rect:area() return self.w * self.h end
local shapes = {Circle.new(1), Rect.new(2, 3), Circle.new(0.5)}
table.sort(shapes, function(a, b) return a:area() < b:area() end)
for _, s in ipairs(shapes) do print(s:describe()) end

-- fila de prioridade (heap)
local Heap = class()
function Heap:init() self.items = {} end
function Heap:push(v)
  local it = self.items
  it[#it + 1] = v
  local i = #it
  while i > 1 do
    local p = i // 2
    if it[p] <= it[i] then break end
    it[p], it[i] = it[i], it[p]
    i = p
  end
end
function Heap:pop()
  local it = self.items
  local top = it[1]
  it[1] = it[#it]
  it[#it] = nil
  local i = 1
  while true do
    local l, r, m = 2 * i, 2 * i + 1, i
    if l <= #it and it[l] < it[m] then m = l end
    if r <= #it and it[r] < it[m] then m = r end
    if m == i then break end
    it[m], it[i] = it[i], it[m]
    i = m
  end
  return top
end
local h = Heap.new()
for _, v in ipairs({5, 3, 9, 1, 7, 2, 8}) do h:push(v) end
local sorted = {}
while #h.items > 0 do sorted[#sorted + 1] = h:pop() end
print(table.concat(sorted, " "))

-- memoização com tabela de chave fraca simulada e varargs
local function memo(f)
  local cache = {}
  return function(n)
    local v = cache[n]
    if v == nil then v = f(n); cache[n] = v end
    return v
  end
end
local fib
fib = memo(function(n) if n < 2 then return n end return fib(n - 1) + fib(n - 2) end)
print(fib(80))
-- contagem de palavras
local text = "o rato roeu a roupa do rei de roma e o rei riu"
local freq = {}
for w in text:gmatch("%a+") do freq[w] = (freq[w] or 0) + 1 end
local words = {}
for w, n in pairs(freq) do words[#words + 1] = {w = w, n = n} end
table.sort(words, function(a, b) if a.n ~= b.n then return a.n > b.n end return a.w < b.w end)
for i = 1, 3 do io.write(words[i].w, "=", words[i].n, " ") end
print()
print(string.format("%-8s|%8.3f|%+d|%#x", "fim", math.pi, 5, 255))
