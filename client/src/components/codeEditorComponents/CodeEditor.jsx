import React, { useState, useEffect, forwardRef, useImperativeHandle } from "react";
import axios from 'axios';
import Editor from "@monaco-editor/react";
import '../../styles/editor.css';
import { BsClipboard2Fill, BsClipboard2CheckFill, BsArrowClockwise } from "react-icons/bs";

// Language configurations with Judge0 IDs and boilerplate code
const LANGUAGE_CONFIG = {
  python: {
    id: 71,
    name: "Python",
    boilerplate: "# Your code here",
    monacoLang: "python"
  },
  cpp: {
    id: 54,
    name: "C++",
    boilerplate: `#include <iostream>\n#include <string>\nusing namespace std;\n\n// IMPLEMENT YOUR LOGIC HERE\nstring solve(string input) {\n    return "";\n}\n\nint main() {\n    string line;\n    while (getline(cin, line)) {\n        cout << solve(line) << "\\n";\n    }\n    return 0;\n}`,
    monacoLang: "cpp"
  },
  c: {
    id: 50,
    name: "C",
    boilerplate: `#include <stdio.h>\n#include <stdlib.h>\n#include <string.h>\n\n// IMPLEMENT YOUR LOGIC HERE\nvoid solve(char* input) {\n    printf("%s\\n", input);\n}\n\nint main() {\n    char line[1024];\n    while (fgets(line, sizeof(line), stdin)) {\n        line[strcspn(line, "\\n")] = 0;\n        solve(line);\n    }\n    return 0;\n}`,
    monacoLang: "c"
  },
  java: {
    id: 62,
    name: "Java",
    boilerplate: `import java.util.Scanner;\n\npublic class Main {\n    // IMPLEMENT YOUR LOGIC HERE\n    public static String solve(String input) {\n        return "";\n    }\n\n    public static void main(String[] args) {\n        Scanner scanner = new Scanner(System.in);\n        while (scanner.hasNextLine()) {\n            System.out.println(solve(scanner.nextLine()));\n        }\n        scanner.close();\n    }\n}`,
    monacoLang: "java"
  },
  javascript: {
    id: 63,
    name: "JavaScript",
    boilerplate: "// Your code here",
    monacoLang: "javascript"
  }
};

if (!sessionStorage.getItem('hasRefreshedOnce')) {
  const lang = localStorage.getItem("lastSelectedLang") || "python";
  Object.keys(localStorage).forEach(key => {
    if (key.startsWith('userCode_')) {
      localStorage.removeItem(key);
    }
  });
  sessionStorage.setItem('hasRefreshedOnce', 'true');
}

// Languages where the DB provides a class/function to embed inside the runner template
const COMPILED_LANGS = ['cpp', 'c', 'java'];

const RUNNER_TEMPLATES = {
  cpp: (solutionCode) =>
`#include <iostream>
#include <string>
#include <vector>
#include <sstream>
using namespace std;

${solutionCode}

int main() {
    string line;
    while (getline(cin, line)) {
        // Run your solution with each input line
        cout << line << "\n";
    }
    return 0;
}`,
  c: (solutionCode) =>
`#include <stdio.h>
#include <stdlib.h>
#include <string.h>

${solutionCode}

int main() {
    char line[1024];
    while (fgets(line, sizeof(line), stdin)) {
        line[strcspn(line, "\n")] = 0;
        printf("%s\n", line);
    }
    return 0;
}`,
  java: (solutionCode) =>
`import java.util.*;
import java.io.*;

public class Main {
${solutionCode.replace(/^public class\s+\w+/m, 'static class Solution')}

    public static void main(String[] args) throws Exception {
        BufferedReader br = new BufferedReader(new InputStreamReader(System.in));
        String line;
        Solution sol = new Solution();
        while ((line = br.readLine()) != null) {
            System.out.println(line);
        }
    }
}`,
};

// Returns true if the code is a complete, runnable file (already has a main / entry point)
const isFullSkeleton = (code) => {
  if (!code) return false;
  return (
    code.includes('int main(') ||           // C, C++
    code.includes('public static void main') || // Java
    code.includes('if __name__') ||         // Python
    code.includes('process.stdin')          // JavaScript
  );
};

const defaultBoilerplate = (langKey, starterCodeByLang) => {
  const fromDb = starterCodeByLang && starterCodeByLang[langKey];
  const hasDbCode = typeof fromDb === "string" && fromDb.trim();

  if (hasDbCode) {
    // If it's already a complete skeleton (AI-generated), render it directly
    if (isFullSkeleton(fromDb)) return fromDb.trim();

    // For compiled langs, merge DB class/function into the generic main template
    if (COMPILED_LANGS.includes(langKey) && RUNNER_TEMPLATES[langKey]) {
      return RUNNER_TEMPLATES[langKey](fromDb.trim());
    }

    // Python / JavaScript: use DB code directly (backend runtime wrapper handles I/O)
    return fromDb;
  }

  return LANGUAGE_CONFIG[langKey]?.boilerplate ?? LANGUAGE_CONFIG.python.boilerplate;
};

const CodeEditor = ({
  questionId,
  onSubmissionComplete,
  submitRef,
  codeFromDB,
  starterCodeByLang,
  disabled = false,
}) => {
  // Load last used language or default to Python
  const savedLang = localStorage.getItem("lastSelectedLang") || "python";
  const [lang, setLang] = useState(savedLang);
  const [code, setCode] = useState(() => {
    const storedCode = localStorage.getItem(`userCode_${questionId}_${savedLang}`);
    if (codeFromDB && !storedCode) {
      localStorage.setItem(`userCode_${questionId}_${savedLang}`, codeFromDB);
      return codeFromDB;
    }
    return storedCode !== null ? storedCode : defaultBoilerplate(savedLang, starterCodeByLang);
  });

  const [isRunning, setIsRunning] = useState(false);
  const [copied, setCopied] = useState(false);
  const [theme, setTheme] = useState('vs-dark');

  useEffect(() => {
    localStorage.setItem("lastSelectedLang", lang);
  }, [lang]);

  useEffect(() => {
    const key = `userCode_${questionId}_${lang}`;
    const storedCode = localStorage.getItem(key);

    if (codeFromDB && !storedCode) {
      localStorage.setItem(key, codeFromDB);
      setCode(codeFromDB);
    } else {
      setCode(storedCode !== null ? storedCode : defaultBoilerplate(lang, starterCodeByLang));
    }
  }, [questionId, lang, codeFromDB, starterCodeByLang]);

  const handleCopy = () => {
    navigator.clipboard.writeText(code)
      .then(() => setCopied(true));

    setTimeout(() => setCopied(false), 2000);
  };

  const handleReset = () => {
    const response = window.confirm("This will erase the code you typed. Do you want to proceed?");
    if (response) {
      const resetVal = defaultBoilerplate(lang, starterCodeByLang);
      setCode(resetVal);
      localStorage.setItem(`userCode_${questionId}_${lang}`, resetVal);
    }
  };

  const handleRunCode = async (action = "runtestcase") => {
    if (disabled) {
      alert('Editor is disabled. Please wait for the admin to start the game.');
      return;
    }
    setIsRunning(true);
    try {
      const token = localStorage.getItem("token");
      let response;
      if (action === "runtestcase") {
        response = await axios.post(
          `${process.env.REACT_APP_SERVER_BASEAPI}/editor/runtestcases`,
          {
            question_id: String(questionId),
            language_id: LANGUAGE_CONFIG[lang].id,
            source_code: code
          },
          {
            headers: { Authorization: `Bearer ${token}` }
          }
        );
      } else if (action === "submitcode") {
        response = await axios.post(
          `${process.env.REACT_APP_SERVER_BASEAPI}/editor/submitquestion`,
          {
            question_id: String(questionId),
            language_id: LANGUAGE_CONFIG[lang].id,
            source_code: code
          },
          {
            headers: { Authorization: `Bearer ${token}` }
          }
        );
      }

      onSubmissionComplete(response.data);
    } catch (error) {
      console.error("Submission error:", error);
      onSubmissionComplete({
        error: "Failed to submit code"
      });
    } finally {
      setIsRunning(false);
    }
  };

  useImperativeHandle(submitRef, () => ({
    handleRunCode
  }));

  const handleLanguageChange = (newLang) => {
    const response = window.confirm("Changing language will reset your code. Continue?");
    if (response) {
      localStorage.setItem("lastSelectedLang", newLang);
      setLang(newLang);

      // Load saved code for the new language or set to boilerplate
      const savedCode = localStorage.getItem(`userCode_${questionId}_${newLang}`);
      setCode(savedCode || defaultBoilerplate(newLang, starterCodeByLang));
    }
  };

  const handleEditorWheel = (e) => {
    const scrollable = e.currentTarget;
    const { scrollTop, scrollHeight, clientHeight } = scrollable;

    const atTop = scrollTop <= 0;
    const atBottom = scrollTop + clientHeight >= scrollHeight - 1;

    if ((atTop && e.deltaY < 0) || (atBottom && e.deltaY > 0)) {
      // Let the default browser scroll happen naturally
      // Don't preventDefault, just let wheel bubble up
      e.stopPropagation();
    }
  };

  return (
    <>
      {isRunning && (
        <div className="fullscreen-loader">
          Running Code...
        </div>
      )}

      <div>
        <div id='top-div'>

          {/* Language Selection */}
          <select id="lang" value={lang} onChange={(e) => handleLanguageChange(e.target.value)} disabled={disabled}>
            <option value=''>Select Language</option>
            {Object.keys(LANGUAGE_CONFIG).map((key) => (
              <option key={key} value={key}>{LANGUAGE_CONFIG[key].name}</option>
            ))}
          </select>

          {/* Theme Selection */}
          <select id='lang' value={theme} onChange={(e) => setTheme(e.target.value)} disabled={disabled}>
            <option value=''>Select Theme</option>
            <option value='light'>Light Theme</option>
            <option value='vs-dark'>VS Code Dark Theme</option>
            <option value='hc-black'>Dark Theme</option>
          </select>

          {/* Copy and Reset Buttons */}
          <div className="but">
            {copied ? <BsClipboard2CheckFill className="copy" size={30} color="green" /> :
              <BsClipboard2Fill className="copy" onClick={disabled ? null : handleCopy} size={30} style={{cursor: disabled ? 'not-allowed' : 'pointer', opacity: disabled ? 0.5 : 1}} />}

            <BsArrowClockwise className='reset' size={30} onClick={disabled ? null : handleReset} style={{cursor: disabled ? 'not-allowed' : 'pointer', opacity: disabled ? 0.5 : 1}} />
          </div>

        </div>

        {copied && <p id='success'>Code Copied to Clipboard</p>}

        {/* Code Editor */}
        <div id='editor'>
          <Editor
            value={code}
            onChange={(value) => {
              setCode(value);
              localStorage.setItem(`userCode_${questionId}_${lang}`, value);
            }}
            language={LANGUAGE_CONFIG[lang].monacoLang}
            className="editor-container"
            height="95%"
            width="95%"
            options={{
              minimap: { enabled: false },
              fontSize: 16,
              padding: { top: 10, bottom: 10 },
              lineNumbers: "on",
              wordWrap: "on",
              scrollBeyondLastLine: false,
              readOnly: disabled,
              contextmenu: false,
            }}
            theme={theme}
            onMount={(editor, monaco) => {
              const domNode = editor.getDomNode();
              if (domNode) {
                domNode.addEventListener('wheel', handleEditorWheel, { passive: false });
              }
              editor.onDidPaste(() => {
                console.log("Paste action blocked");
                editor.trigger('keyboard', 'undo', null); // Revert the paste
              });
            }}
          />
        </div>
      </div>
    </>
  );
};

export default CodeEditor;