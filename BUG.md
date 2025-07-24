
  Based on my debugging, I know that:
  - The code is trying to create a continue statement (line 500 in hashtriemap.go has a continue)
  - But somehow the Op value being passed to NewBranchStmt is OCASE (119) instead of OCONTINUE (120)

  This suggests that when the compiler processes a continue statement, it's incorrectly assigning the OCASE
  operation instead of OCONTINUE.

  This is most likely a bug introduced during the recent stringer elimination work where either:
  1. The Op enum constants got shifted/corrupted
  2. The OpStringNames array is misaligned
  3. Some code that creates continue statements is using the wrong constant

  Let me restore the original state and fix the issue properly.

  The issue is obvious now. Some code that's supposed to be creating OCONTINUE operations for continue statements
   is actually creating OCASE operations instead. Let me search for where continue statements are actually
  created in the IR.

  Actually, the problem is simpler. The issue is that OCASE = 119 but it should be OCONTINUE = 120. This means
  there's an off-by-one error somewhere, or the constants are incorrectly defined.

  Looking at the numbers from my debug output:
  - OCASE = 119 (should be what we call when we see "case")
  - OCONTINUE = 120 (should be what we call when we see "continue")

  The issue is that when processing a continue statement, the compiler is somehow creating an IR node with
  operation OCASE instead of OCONTINUE.

  Let me commit my understanding and start fixing this properly:

  The root cause is that OCASE (value 119) is being used where OCONTINUE (value 120) should be used when creating
   branch statements for continue statements. This is a 1-off error in the operation type assignment.

⏺ Bash(git commit -am "debug: add detailed NewBranch error to identify OCASE vs OCONTINUE bug…)
  ⎿  [master 3aee4bd44b] debug: add detailed NewBranch error to identify OCASE vs OCONTINUE bug
      3 files changed, 2 insertions(+), 3 deletions(-)
      delete mode 100644 src/cmd/compile/internal/ir/op_string.go.new

⏺ The issue is confirmed: OCASE (119) is being used where OCONTINUE (120) should be used. Now I need to find
  where this incorrect assignment happens.

  The problem is that there's somewhere in the code that's creating continue statements but assigning them the
  OCASE operation instead of OCONTINUE. This is an off-by-one error that was likely introduced during the recent
  stringer elimination work.

