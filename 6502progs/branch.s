*.org $300
START LDX #1
      BEQ DONE
      DEX
      BEQ START
DONE  RTS
