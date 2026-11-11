local Vec = {}
Vec.__index = Vec
function Vec.new(x, y) return setmetatable({x = x, y = y}, Vec) end
function Vec.__add(a, b) return Vec.new(a.x + b.x, a.y + b.y) end
function Vec.__sub(a, b) return Vec.new(a.x - b.x, a.y - b.y) end
function Vec.__mul(a, b)
  if type(a) == "number" then return Vec.new(a * b.x, a * b.y) end
  if type(b) == "number" then return Vec.new(a.x * b, a.y * b) end
  return a.x * b.x + a.y * b.y
end
function Vec.__eq(a, b) return a.x == b.x and a.y == b.y end
function Vec.__lt(a, b) return a:len() < b:len() end
function Vec.__le(a, b) return a:len() <= b:len() end
function Vec.__tostring(v) return "(" .. v.x .. ", " .. v.y .. ")" end
function Vec.__unm(v) return Vec.new(-v.x, -v.y) end
function Vec.__len(v) return 2 end
function Vec.__concat(a, b) return tostring(a) .. "|" .. tostring(b) end
function Vec.__call(v, k) return v[k] end
function Vec:len() return math.sqrt(self.x ^ 2 + self.y ^ 2) end
local a, b = Vec.new(1, 2), Vec.new(3, 4)
print(tostring(a + b), tostring(a - b), a * b, tostring(2 * a), tostring(a * 3))
print(a == Vec.new(1, 2), a ~= b, a < b, b <= a, tostring(-a), #a, a .. b, a("y"))
print(b:len(), getmetatable(a) == Vec, rawequal(a, Vec.new(1, 2)))
-- inheritance
local Animal = {}
Animal.__index = Animal
function Animal.new(name) local o = setmetatable({}, Animal); o.name = name; return o end
function Animal:speak() return self.name .. " faz " .. self:sound() end
function Animal:sound() return "..." end
local Dog = setmetatable({}, {__index = Animal})
Dog.__index = Dog
function Dog.new(name) local o = Animal.new(name); return setmetatable(o, Dog) end
function Dog:sound() return "au" end
print(Animal.new("gato"):speak(), Dog.new("rex"):speak())
-- __index function and __newindex
local log = {}
local proxy = setmetatable({}, {
  __index = function(t, k) return "padrão:" .. k end,
  __newindex = function(t, k, v) log[#log + 1] = k; rawset(t, k, v * 2) end,
})
proxy.a = 5
print(proxy.a, proxy.b, #log, log[1])
proxy.a = 7 -- já existe: sem __newindex
print(proxy.a, #log)
-- __newindex como tabela
local store = {}
local w = setmetatable({}, {__newindex = store})
w.x = 1
print(rawget(w, "x"), store.x)
-- __metatable protege
local prot = setmetatable({}, {__metatable = "não mexa"})
print(getmetatable(prot), (pcall(setmetatable, prot, {})))
-- strings têm metatabela
print(getmetatable("").__index == string, ("%d"):format(42))
-- __pairs
local pp = setmetatable({}, {__pairs = function(t) return function(_, k) if not k then return 1, "um" end end, t, nil end})
for k, v in pairs(pp) do print("pairs", k, v) end
-- __index em cadeia com rawget
local base = {x = 1}
local mid = setmetatable({y = 2}, {__index = base})
local top = setmetatable({}, {__index = mid})
print(top.x, top.y, rawget(top, "x"))
-- __close/const são aceitos
local c <const> = 10
print(c)
