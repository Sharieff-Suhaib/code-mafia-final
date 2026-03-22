import React, { useEffect, useState, useRef } from 'react';
import CodeEditor from '../components/codeEditorComponents/CodeEditor.jsx';
import CodeDescriptionPane from '../components/codeEditorComponents/CodeDescriptionPane.jsx';
import BottomPanel from '../components/codeEditorComponents/BottomPanel.jsx';
import Navbar from '../components/Navbar.jsx';
import '../styles/editorPage.css'
import { BsArrowBarUp } from "react-icons/bs";
import { BsArrowBarDown } from "react-icons/bs";
import PowerupsDialog from '../components/powerUpComponents/PowerupsDialog.jsx';
import axios from "axios";
import PowerUpController from '../components/powerUpComponents/PowerUpController.jsx';
import TestCases from '../components/codeEditorComponents/TestCases.jsx';
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import PopupMessage from '../components/PopupMessage.jsx';
import { getTeams } from '../components/Store/store.js';
import PowerUpTimer from '../components/powerUpComponents/PowerUpTimer.jsx';

/** Go backend sends `test_cases` as an array; legacy shape was an object map. */
function normalizeTestCasesForPanel(testCases) {
    if (!testCases) return [];
    if (Array.isArray(testCases)) {
        return testCases.map((tc) => ({
            id: tc.name,
            input: tc.input,
            expected_output: tc.expected_output,
        }));
    }
    return Object.entries(testCases).map(([key, value]) => ({
        id: key,
        input: value.input,
        expected_output: value.expected_output,
    }));
}

const EditorPage = () => {

    const [testCaseList, setTestCaseList] = useState([]);
    const [problemTitle, setProblemTitle] = useState("");
    const [problemDifficulty, setProblemDifficulty] = useState("");
    const [problemDescription, setProblemDescription] = useState("");
    // aiStarterCodes: { [challengeId]: { python: '...', cpp: '...', ... } }
    const [aiStarterCodes, setAiStarterCodes] = useState({});

    const [questionSet, setQuestionSet] = useState([]);

    const [currentQuestion, setCurrentQuestion] = useState(1);
    const [totalQuestions, setTotalQuestions] = useState(10);
    const [xp, setXp] = useState(0);

    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [gameStatus, setGameStatus] = useState('stopped');
    const [editorEnabled, setEditorEnabled] = useState(false);
    const [statusMessage, setStatusMessage] = useState('');


    const {
        powers,
        teams,
        coins,
        powerupPopupOpen,
        powerupsDialogOpen,
        message,
        activePowerUps,
        getCoins,
        setClickedPower,
        setClickedTeam,
        handleApply,
        refreshUsers,
        setPowerupPopupOpen,
        setPowerupsDialogOpen,
        overlayRef
    } = PowerUpController();

    const submitRef = useRef();

    // Fetch AI-generated starter codes for a specific challenge
    const fetchAIStarterCode = async (challengeId) => {
        // Don't re-fetch if already loaded
        if (aiStarterCodes[challengeId]) return;
        try {
            const response = await fetch(
                `${process.env.REACT_APP_SERVER_BASEAPI}/ai/starter?challengeId=${challengeId}`
            );
            if (!response.ok) return;
            const data = await response.json();
            if (data.starter_code) {
                setAiStarterCodes(prev => ({ ...prev, [challengeId]: data.starter_code }));
            }
        } catch (err) {
            console.warn('AI starter code fetch failed:', err);
        }
    };

    const loadQuestion = async () => {
        const question = questionSet[currentQuestion - 1];
        if (!question) return;

        setProblemTitle(`${currentQuestion}. ${question.title}`);
        setProblemDifficulty(question.difficulty);
        setProblemDescription(question.description);
        setTestCaseList(normalizeTestCasesForPanel(question.test_cases));
        // Kick off AI starter code fetch in the background
        fetchAIStarterCode(question.id);
    };


    const getSubmissionStatus = async () => {
        try {
            const response = await axios.get(`${process.env.REACT_APP_SERVER_BASEAPI}/problem/status`, {
                headers: {
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
            });

            const statusMap = response.data;

            // Update the questionSet with status and code information
            setQuestionSet(prevQuestions =>
                prevQuestions.map(question => ({
                    ...question,
                    status: statusMap[question.id]?.status || 'unattempted',
                    code: statusMap[question.id]?.code || ''
                }))
            );
        } catch (error) {
            console.error('Error fetching submission status:', error);
            // If the request fails, set all questions to 'unattempted' and code to ''
            setQuestionSet(prevQuestions =>
                prevQuestions.map(question => ({
                    ...question,
                    status: 'unattempted',
                    code: ''
                }))
            );
        }
    };

    const getXP = async () => {
        axios.get(`${process.env.REACT_APP_SERVER_BASEAPI}/editor/points`, {
            headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
        }).then(response => {
            if (response.data && response.data.points !== undefined) {
                setXp(response.data.points);
            }
        }).catch(err => {
            console.error("Failed to fetch points:", err);
        });
    }

    // Use a ref to track previous status to avoid dependency issues
    const previousStatusRef = useRef('stopped');

    const checkGameStatus = async () => {
        try {
            const response = await axios.get(`${process.env.REACT_APP_SERVER_BASEAPI}/game/status`);
            const status = response.data.status;

            console.log('Game status check:', status, 'Previous:', previousStatusRef.current);

            // Check if status changed and show notification
            const previousStatus = previousStatusRef.current;

            if (status === 'start') {
                setEditorEnabled(true);
                setStatusMessage('✅ Game is Active - Editor Enabled');

                // Show notification only when status changes from non-start to start
                if (previousStatus !== 'start') {
                    console.log('Game started! Showing notification');
                    // Use a more prominent notification
                    const notification = document.createElement('div');
                    notification.style.cssText = `
                        position: fixed;
                        top: 80px;
                        left: 50%;
                        transform: translateX(-50%);
                        background: #4CAF50;
                        color: white;
                        padding: 20px 40px;
                        border-radius: 8px;
                        font-size: 18px;
                        font-weight: bold;
                        z-index: 10000;
                        box-shadow: 0 4px 6px rgba(0,0,0,0.3);
                        animation: slideDown 0.5s ease-out;
                    `;
                    notification.textContent = '🎮 GAME STARTED! The editor is now enabled. You can start coding!';
                    document.body.appendChild(notification);

                    // Remove notification after 5 seconds
                    setTimeout(() => {
                        notification.style.animation = 'slideUp 0.5s ease-out';
                        setTimeout(() => notification.remove(), 500);
                    }, 5000);
                }
            } else if (status === 'pause') {
                setEditorEnabled(false);
                setStatusMessage('⏸️ Game is paused. Editor is disabled.');

                // Show notification when game is paused
                if (previousStatus === 'start') {
                    alert('⏸️ Game has been paused by the admin.');
                }
            } else if (status === 'stop' || status === 'stopped') {
                setEditorEnabled(false);
                setStatusMessage('🔒 Game has been stopped. Editor is disabled.');

                // Show notification when game is stopped
                if (previousStatus === 'start' || previousStatus === 'pause') {
                    alert('🛑 Game has been stopped by the admin.');
                }
            } else {
                setEditorEnabled(false);
                setStatusMessage('🔒 Game has not started yet. Please wait for admin to start the game.');
            }

            // Update the ref with current status
            previousStatusRef.current = status;
            setGameStatus(status);
        } catch (error) {
            console.error('Error checking game status:', error);
            setEditorEnabled(false);
            setStatusMessage('Unable to check game status. Editor is disabled.');
        }
    };


    const onSubmissionComplete = (results) => {
        if (results.error) {
            return;
        }

        getXP();
        getCoins();


        setTestCaseList(results.results.map(result => ({
            name: result.testCase,
            input: result.input,
            expected_output: result.expectedOutput,
            output: result.output,
            status: result.status,
            stderr: result.stderr,
            compileOutput: result.compileOutput,
            message: result.message,
        })));
        if (results.summary) {
            console.log("Submission summary:", results.summary, results.passed, "/", results.total);
        }
    }

    useEffect(() => {
        const verifyToken = async () => {
            const token = localStorage.getItem('token');
            if (token) {
                try {
                    const baseURL = process.env.REACT_APP_SERVER_BASEAPI.replace('/api', '');
                    const response = await axios.get(`${baseURL}/auth/verify`, {
                        headers: { Authorization: `Bearer ${token}` }
                    });
                    if (!response.data.valid) {
                        window.location.href = "/login"
                    }
                } catch (error) {
                    console.error('Token verification failed');
                    window.location.href = "/login"
                }
            } else {
                window.location.href = "/login";
            }
        };
        verifyToken();
        getXP();
        getCoins();
        checkGameStatus();

        // Poll game status every 5 seconds
        const statusInterval = setInterval(checkGameStatus, 5000);

        return () => clearInterval(statusInterval);
    }, []);

    useEffect(() => {
        const fetchQuestions = async () => {
            try {
                setLoading(true);
                const response = await fetch(`${process.env.REACT_APP_SERVER_BASEAPI}/problem`);
                if (!response.ok) {
                    throw new Error("Failed to fetch questions");
                }
                const data = await response.json();

                if (!data.qs || !Array.isArray(data.qs) || data.qs.length === 0) {
                    throw new Error("No questions available.");
                }
                setQuestionSet(data.qs);
                setTotalQuestions(data.qs.length);
                setCurrentQuestion(1);
            } catch (err) {
                setError(err.message);
            } finally {
                setLoading(false);
            }
        }
        fetchQuestions();
        getSubmissionStatus();
    }, []);

    useEffect(() => {
        if (questionSet.length > 0) {
            loadQuestion();
        }
    }, [currentQuestion, questionSet]);


    const gotoNextQuestion = () => {
        setCurrentQuestion(prev => Math.min(prev + 1, totalQuestions));
    };

    const gotoPrevQuestion = () => {
        setCurrentQuestion(prev => Math.max(prev - 1, 1));
    };

    const [windowWidth, setWindowWidth] = useState(window.innerWidth);

    useEffect(() => {
        const handleResize = () => setWindowWidth(window.innerWidth);
        window.addEventListener("resize", handleResize);
        return () => window.removeEventListener("resize", handleResize);
    }, []);

    const [open, setOpen] = useState(false)

    if (loading) return <p>Loading...</p>;
    if (error) return <p>Error: {error}</p>;
    if (!questionSet) return <p>No questions available.</p>;
    return (
        <>
            <div id="overlay" ref={overlayRef}></div>

            <div className='main'>
                <div>
                    <Navbar />
                </div>

                {/* Game Status Banner */}
                {!editorEnabled && statusMessage && (
                    <div style={{
                        backgroundColor: '#dc3545',
                        color: 'white',
                        padding: '15px',
                        textAlign: 'center',
                        fontFamily: '"Press Start 2P", cursive',
                        fontSize: '0.8rem',
                        borderBottom: '3px solid #000',
                        boxShadow: '0 4px 0 #000'
                    }}>
                        🔒 {statusMessage}
                    </div>
                )}

                {editorEnabled && (
                    <div style={{
                        backgroundColor: '#28a745',
                        color: 'white',
                        padding: '10px',
                        textAlign: 'center',
                        fontFamily: '"Press Start 2P", cursive',
                        fontSize: '0.7rem',
                        borderBottom: '3px solid #000',
                        boxShadow: '0 4px 0 #000'
                    }}>
                        ✅ Game is Active - Editor Enabled
                    </div>
                )}

                <div>
                    <div className="content" style={{ height: '100vh' }}>
                        <PanelGroup autoSaveId="codePanelLayout" direction='horizontal'>
                            <Panel defaultSize={50}>
                                {/* Left Pane */}
                                <div className="left-pane" style={{ paddingRight: '10px', height: '100%' }}>
                                    <div className="desc">
                                        <CodeDescriptionPane
                                            problemTitle={problemTitle}
                                            problemDescription={problemDescription}
                                            problemDifficulty={problemDifficulty}
                                        />
                                    </div>

                                    <div id="test-case-choose">
                                        <TestCases testCases={testCaseList} />
                                    </div>
                                </div>
                            </ Panel>
                            <PanelResizeHandle className="panelresizer" />
                            <Panel defaultSize={50}>
                                {/* Right Pane */}
                                <div className="right-pane" style={{ height: '100%' }}>
                                    <div className="editor">
                                        <CodeEditor
                                            questionId={questionSet[currentQuestion - 1].id}
                                            onSubmissionComplete={(results) => onSubmissionComplete(results)}
                                            submitRef={submitRef}
                                            codeFromDB={questionSet[currentQuestion - 1].code}
                                            starterCodeByLang={
                                                // Prefer AI-generated starter code; fall back to DB starter_code
                                                aiStarterCodes[questionSet[currentQuestion - 1].id] ||
                                                questionSet[currentQuestion - 1].starter_code
                                            }
                                            disabled={!editorEnabled}
                                        />
                                    </div>
                                </div>
                            </Panel>
                        </PanelGroup>
                    </div>

                </div>
                <div>
                    {windowWidth < 770 ? (
                        <div
                            style={{
                                position: "fixed",
                                bottom: open ? "20px" : "20px",
                                left: "10%",
                                transform: "translateX(-50%)",
                                zIndex: 1000,
                                display: "flex",
                                justifyContent: "center",
                                alignItems: "center",
                                width: "100%",
                                pointerEvents: "none",
                            }}
                        >
                            <div
                                style={{
                                    backgroundColor: "#FFD400",
                                    borderRadius: "50%",
                                    padding: "8px",
                                    boxShadow: "0 4px 12px rgba(0, 0, 0, 0.2)",
                                    pointerEvents: "auto",
                                    cursor: "pointer",
                                }}
                                onClick={() => setOpen(!open)}
                            >
                                {open ? (
                                    <BsArrowBarDown size={24} color="#030617" />
                                ) : (
                                    <BsArrowBarUp size={24} color="#030617" />
                                )}
                            </div>
                        </div>
                    ) : null}

                    {windowWidth >= 770 || open ? (
                        <BottomPanel
                            currentQuestion={currentQuestion}
                            totalQuestions={totalQuestions}
                            xp={xp}
                            setCurrentQuestion={setCurrentQuestion}
                            gotoNextQuestion={gotoNextQuestion}
                            gotoPrevQuestion={gotoPrevQuestion}
                            powerupsDialogOpen={powerupsDialogOpen}
                            setPowerupsDialogOpen={(val) => {
                                if (val) refreshUsers(); // refresh live teams on open
                                setPowerupsDialogOpen(val);
                            }}
                            questions={questionSet}
                            submitRef={submitRef}
                        />
                    ) : null}

                    <PowerUpTimer activePowerUps={activePowerUps} powersList={powers} />

                    {powerupsDialogOpen &&
                        <PowerupsDialog
                            onClose={() => setPowerupsDialogOpen(false)}
                            powers={powers}
                            teams={teams || []}
                            onPowerSelect={setClickedPower}
                            onTeamSelect={setClickedTeam}
                            onUsePower={handleApply}
                            coins={coins} />
                    }
                    {powerupPopupOpen &&
                        <PopupMessage
                            onClose={() => setPowerupPopupOpen(false)}
                            message={message}
                        />
                    }
                </div>

            </div>
        </>
    );
}

export default EditorPage;


