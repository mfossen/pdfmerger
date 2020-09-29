## Running on Windows

1. Download `pdfmerger.exe` from the [releases page](https://github.com/mfossen/pdfmerger/releases/tag/v0.1) page, or directly https://github.com/mfossen/pdfmerger/releases/download/v0.1/pdfmerger.exe
2. Open PowerShell by hitting the windows key and typing in `powershell` in the windows start menu.
3. Navigate to where you downloaded `pdfmerger.exe`, if it's in your Downloads folder, typing `cd Downloads` in PowerShell should get you there.
4. Run `pdfmerger.exe` by typing `./pdfmerger.exe --help` to display a list of options.

## Combining PDF files

1. `pdfmerger` will read in a directory of PDF files, group them by `T_##` and then merge them in sorted order into a single file named `T_##.pdf`
2. `pdfmerger.exe --input-directory in-pdfs --output-directory out-pdfs` should be all the program needs, and will create `out-pdfs` if it doesn't exist.
    i. for example, if you are in powershell in your Downloads folder where you downloaded `pdfmerger.exe`, with a directory of PDF files in the Downloads folder, you'd run `./pdfmerger.exe --input-directory 'Input PDF Files' --output-directory 'Output PDF Files'`
