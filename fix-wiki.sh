# git fetch --all
git pull

# ONLY NEW ONES last 3 days vs ALL!
# for f in `find . -maxdepth 1 -mtime -3`; do
for f in *.md; do
	sed -E -i '' -e 's/[[:space:]]*$/  /' "$f"
done
# ./repair-links.js not necessary?
git commit -a --allow-empty-message -m '⇔'
git push

# broken-link-checker "http://pannous.github.io/hieros/Home" -ro --get |gv 400|gv "─OK─"