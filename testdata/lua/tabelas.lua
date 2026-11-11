local t = {5, 2, 8, 1, 9}
table.sort(t)
print(table.concat(t, " "))
table.sort(t, function(a, b) return a > b end)
print(table.concat(t, " "))
local names = {"carla", "ana", "bruno"}
table.sort(names)
print(table.concat(names, ","))
table.insert(t, 7)
table.insert(t, 1, 0)
print(table.concat(t, " "), #t)
print(table.remove(t), table.remove(t, 1), table.concat(t, " "), #t)
print(table.concat({}, ","), table.concat({1, 2, 3}, ", ", 2, 3))
local p = table.pack(1, nil, 3)
print(p.n, p[1], p[2], p[3])
local m = table.move({1, 2, 3}, 1, 3, 2)
print(table.concat(m, ","))
local m2 = table.move({1, 2, 3}, 1, 3, 1, {})
print(table.concat(m2, ","))
-- remove items while iterating with pairs
local d = {a = 1, b = 2, c = 3, d = 4}
for k in pairs(d) do d[k] = nil end
print(next(d))
-- length and holes
local h = {}
for i = 1, 10 do h[i] = i end
print(#h)
h[10] = nil
print(#h)
for i = 1, 9 do h[i] = nil end
print(#h, next(h))
-- keys of various types
local key = {}
local f = function() end
local k = {[key] = "tabela", [f] = "função", [1.0] = "um", [true] = "verdade", ["1"] = "texto"}
print(k[key], k[f], k[1], k[true], k["1"])
-- nested
local nested = {a = {b = {c = "fundo"}}}
print(nested.a.b.c, nested["a"]["b"]["c"])
-- table equality is identity
print({} == {}, t == t)
-- integer float keys unify
local u = {}
u[2] = "dois"
print(u[2.0], u[2])
-- insert in loop and counting
local big = {}
for i = 1, 1000 do big[#big + 1] = i * i end
print(#big, big[1000])
local cnt = 0
for _ in pairs(big) do cnt = cnt + 1 end
print(cnt)
-- ipairs stops at nil
local holes = {1, 2, nil, 4}
local c2 = 0
for _ in ipairs(holes) do c2 = c2 + 1 end
print(c2)
-- next with explicit loop
local sumk = 0
local kk, vv = next({10, 20, 30})
print(kk, vv)
print(select(-1, 1, 2, 3), select(2, 1, 2, 3))
-- rawlen / rawget / rawset
local r = setmetatable({}, {__len = function() return 99 end})
r[1] = "x"
print(#r, rawlen(r), rawget(r, 1))
