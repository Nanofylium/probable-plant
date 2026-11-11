-- Jogo da velha em Lua 5.4, rodando no nep:  nep exemplos/lua/jogo_da_velha.lua
-- O computador joga contra ele mesmo com minimax (perfeito: sempre empata).

local Tabuleiro = {}
Tabuleiro.__index = Tabuleiro

function Tabuleiro.novo()
  return setmetatable({casas = {" ", " ", " ", " ", " ", " ", " ", " ", " "}}, Tabuleiro)
end

local linhas = {{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {1, 4, 7}, {2, 5, 8}, {3, 6, 9}, {1, 5, 9}, {3, 5, 7}}

function Tabuleiro:vencedor()
  for _, l in ipairs(linhas) do
    local a, b, c = self.casas[l[1]], self.casas[l[2]], self.casas[l[3]]
    if a ~= " " and a == b and b == c then return a end
  end
  for _, v in ipairs(self.casas) do if v == " " then return nil end end
  return "empate"
end

function Tabuleiro:__tostring()
  local c = self.casas
  return string.format(" %s | %s | %s\n---+---+---\n %s | %s | %s\n---+---+---\n %s | %s | %s",
    table.unpack(c))
end

local function minimax(t, vez)
  local v = t:vencedor()
  if v == "X" then return 1 elseif v == "O" then return -1 elseif v then return 0 end
  local melhor, jogada = (vez == "X") and -2 or 2, nil
  for i = 1, 9 do
    if t.casas[i] == " " then
      t.casas[i] = vez
      local nota = minimax(t, vez == "X" and "O" or "X")
      t.casas[i] = " "
      if (vez == "X" and nota > melhor) or (vez == "O" and nota < melhor) then
        melhor, jogada = nota, i
      end
    end
  end
  return melhor, jogada
end

-- cada jogada vem de uma corrotina
local partida = coroutine.wrap(function()
  local t, vez = Tabuleiro.novo(), "X"
  while not t:vencedor() do
    local _, i = minimax(t, vez)
    t.casas[i] = vez
    coroutine.yield(vez, i, t)
    vez = (vez == "X") and "O" or "X"
  end
end)

local ultimo
for vez, casa, t in partida do
  print(("%s joga na casa %d"):format(vez, casa))
  ultimo = t
end
print(tostring(ultimo))
print("Resultado: " .. ultimo:vencedor())
