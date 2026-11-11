local f = load("return 1 + 2")
print(f())
local g = load("local a, b = ... return a * b")
print(g(6, 7))
print((load("syntax error here")), type(select(2, load("x x"))))
local env = {x = 5}
local h = load("return x", "chunk", "t", env)
print(h())
local parts = {"return ", "'pe", "daços'"}
local i = 0
print(load(function() i = i + 1; return parts[i] end)())
x_global = 10
print(load("return x_global")())
_G.y_global = 20
print(y_global, _G.x_global, _ENV == _G)
local count = 0
for k, v in pairs(_G) do if k == "x_global" then count = count + 1 end end
print(count, _VERSION)
local mod = require("modulo_teste")
print(mod.dobro(21), mod.nome, require("modulo_teste") == mod, package.loaded.modulo_teste == mod)
print(type(os.time()), os.date("%Y-%m-%d", 0) ~= nil, os.time({year = 2026, month = 1, day = 1, hour = 0}) - os.time({year = 2025, month = 12, day = 31, hour = 0}))
print(os.date("!%Y-%m-%d %H:%M:%S", 86400 * 365), os.date("!*t", 0).year, type(os.clock()))
print(string.format("%s", setmetatable({}, {__tostring = function() return "objeto" end})))
