# Regression Analysis After Merge

## Current Status
- **Before merge (8ea19c53ac)**: 114/122 passed (93.4%)
- **After merge (current)**: 75/118 passed (63.6%)
- **Net regression**: 39 fewer passing tests

## Tests Fixed by Merge
These tests were failing before but now pass:
- test_in_operator_auto_import.goo ✅
- test_interpolation.goo ✅
- test_string_interpolation.goo ✅
- test_string_var_spacing.goo ✅

## New Failures (40 tests)
Tests that passed before merge but now fail:
1. test_array_like_slice.goo
2. test_as_cast_convert.goo
3. test_check_reverse.goo
4. test_class_methods.goo
5. test_ellipsis.goo
6. test_filter_synonyms.goo
7. test_for_loop.goo
8. test_hash_index.goo
9. test_in_operator_rune_strings.goo
10. test_in_operator_strings.goo
11. test_iterator_for_in.goo
12. test_iterator_membership.goo
13. test_iterator_simple.goo
14. test_list_comparison.goo
15. test_list_comparison2.goo
16. test_list_equality.goo
17. test_list_filter.goo
18. test_list_methods_broken.goo
19. test_list_methods.goo
20. test_list_typed.goo
21. test_manual_strings.goo
22. test_map_dot_nested.goo
23. test_map.goo
24. test_minimal_conflict.goo
25. test_modify.goo
26. test_return_void.goo
27. test_slice_inference_core.goo
28. test_string_concat.goo
29. test_string_methods_todo.goo
30. test_string_methods.goo
31. test_string_replace.goo
32. test_string_reverse.goo
33. test_strings_auto_import.goo
34. test_tau_pi_approx.goo
35. test_transform_synonyms.goo
36. test_truthy_and.goo
37. test_truthy.goo
38. test_try_assign_context_aware.goo
39. test_try_assign.goo
40. test_typeof.goo

## Attempted Fixes
1. **Restored updateImportConfigForTransforms** in src/cmd/compile/internal/noder/unified.go
   - This was changed to "do nothing" during merge
   - Restored pre-merge implementation
   - Result: No improvement (still 75/118)
   - Conclusion: Import config wasn't the root cause

## Observed Error Patterns
1. **typeof test**: Returns "interface {}" instead of "untyped int"
2. **truthy tests**: "non-boolean condition in if statement" - suggests truthy transform not working
3. **string methods**: Import errors for auto-added imports

## Next Steps Needed
The root cause appears to be in how transformers are being applied or processed, not in the import resolution. Need to:
1. Check if transformers are being registered/applied correctly
2. Verify transform pipeline hasn't changed
3. Check for changes in types2 or noder that affect how transformed code is processed

## Files Changed in Merge (src/cmd/compile/internal/noder/)
- unified.go: updateImportConfigForTransforms disabled (now restored)
- reader.go: Minor renaming (defi => def) - cosmetic
- writer.go: interface{} => any, new(expr) support - shouldn't break transforms
- doc.go: Comment typo fixes - no functional impact
