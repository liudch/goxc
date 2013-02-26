for D in dl/*; do
	if [ -d "${D}" ]; then
		echo "${D}"   # your processing here
		cat _downloads_header.md "${D}/downloads.md" > "${D}/index.md"
	fi
done

cat _index_header.md README.md > index.md
jekyll
