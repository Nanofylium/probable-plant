-- closures, upvalues, varargs, multiple results
local fs = {}
for i = 1, 3 do fs[i] = function() return i end end
print(fs[1](), fs[2](), fs[3]())
local acc = {}
local j = 0
while j < 3 do j = j + 1; local k = j; acc[#acc+1] = function() k = k + 10; return k end end
print(acc[1](), acc[1](), acc[2]())
local function counter()
  local n = 0
  return function() n = n + 1; return n end, function() return n end
end
local inc, get = counter()
inc(); inc()
print(get())
local function va(...)
  local a, b = ...
  return select('#', ...), a, b, ...
end
print(va())
print(va(1, nil, 3))
print((va(1, 2)))
local function pack2(...) return {n = select('#', ...), ...} end
local p = pack2(nil, 2, nil)
print(p.n, p[1], p[2], p[3])
print(table.unpack({1, 2, 3}))
print(table.unpack({1, 2, 3}, 2))
print(string.format("%d-%d", (function() return 4, 5 end)()))
local t = {(function() return 1, 2 end)(), (function() return 3, 4 end)()}
print(#t, t[1], t[2], t[3])
-- tail calls do not grow the stack
local function loop(n) if n == 0 then return "fim" end return loop(n - 1) end
print(loop(100000))
-- recursive local function
local function fact(n) if n <= 1 then return 1 end return n * fact(n - 1) end
print(fact(20))
-- multiple assignment and swap
local x, y = 1, 2
x, y = y, x
print(x, y)
local tt = {}
local i = 1
i, tt[i] = i + 1, 20
print(i, tt[1], tt[2])
-- nested functions with shared upvalue
local function outer()
  local v = 0
  local function a() v = v + 1 end
  local function b() return v end
  return a, b
end
local a1, b1 = outer()
a1(); a1(); a1()
print(b1())
-- string methods via colon
local s = "hello"
print(s:upper(), s:len(), ("x"):rep(3, ","), #s)
-- varargs in table constructor and in the middle
local function f3() return 1, 2, 3 end
print(#{f3(), f3()}, #{f3(), (f3())}, #{f3(), nil})
print(math.max(f3()), math.min(f3()))
