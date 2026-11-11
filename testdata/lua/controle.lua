-- goto, repeat, numeric for edge cases, break
for i = 1, 3 do
  for j = 1, 3 do
    if j == 2 then goto continue end
    io.write(i, j, " ")
    ::continue::
  end
end
print()
do
  local k = 0
  ::top::
  k = k + 1
  if k < 3 then goto top end
  print("k", k)
end
local n = 0
repeat local m = n; n = n + 1 until m >= 3
print("n", n)
for i = 10, 1, -3 do io.write(i, " ") end print()
for i = 1, 2, 0.5 do io.write(i, " ") end print()
for i = 1.0, 3 do io.write(i, " ") end print()
for i = 3, 1 do io.write("nunca") end print("vazio")
for i = math.maxinteger - 2, math.maxinteger do io.write(i, " ") end print()
for i = 1, 3.7 do io.write(i, " ") end print()
local r = {}
for i = 1, 10 do if i > 4 then break end r[#r + 1] = i end
print(table.concat(r, ","))
local w = 0
while true do w = w + 1; if w == 5 then break end end
print("w", w)
-- if/elseif
local function cls(x)
  if x < 0 then return "neg" elseif x == 0 then return "zero" else return "pos" end
end
print(cls(-1), cls(0), cls(1))
-- and/or values
print(nil or "a", false and 1, 0 and "zero é verdadeiro", "" and "vazio também", nil and nil)
print(not nil, not 0, 1 and 2 or 3)
-- generic for with custom iterator and state
local function range(n)
  return function(s, c) if c < s then return c + 1 end end, n, 0
end
local sum = 0
for v in range(5) do sum = sum + v end
print("soma", sum)
-- nested break in repeat
local cnt = 0
repeat cnt = cnt + 1; if cnt == 3 then break end until false
print("cnt", cnt)
-- closures capture loop var in while via locals
local fns = {}
for _, name in ipairs({"a", "b"}) do fns[#fns + 1] = function() return name end end
print(fns[1](), fns[2]())
