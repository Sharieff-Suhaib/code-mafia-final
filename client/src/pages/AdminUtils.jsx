import React, { useState, useEffect } from 'react';
import axios from 'axios';
import '../styles/AdminUtils.css';

const AdminUtils = () => {
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [gameStatus, setGameStatus] = useState('stopped');
  const [totalTeams, setTotalTeams] = useState(0);
  const [teams, setTeams] = useState([]);
  const [challenges, setChallenges] = useState([]);

  // Problem upload form
  const [problemForm, setProblemForm] = useState({
    id: '',
    title: '',
    description: '',
    difficulty: 'easy',
    points: 10,
    test_cases: {}
  });

  // Test case form
  const [testCaseForm, setTestCaseForm] = useState({
    name: '',
    input: '',
    expected_output: '',
    type: 'visible'
  });

  const [testCases, setTestCases] = useState([]);
  const [newTeamName, setNewTeamName] = useState('');

  useEffect(() => {
    fetchGameStatus();
    fetchTeams();
    fetchChallenges();
  }, []);

  const fetchGameStatus = async () => {
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/game/status`,
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );
      setGameStatus(response.data.status);
      setTotalTeams(response.data.totalTeams);
    } catch (error) {
      console.error('Error fetching game status:', error);
    }
  };

  const fetchTeams = async () => {
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/teams`,
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );
      setTeams(response.data.teams || []);
    } catch (error) {
      console.error('Error fetching teams:', error);
    }
  };

  const fetchChallenges = async () => {
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/problems`,
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );
      setChallenges(response.data.challenges || []);
    } catch (error) {
      console.error('Error fetching challenges:', error);
    }
  };

  const handleCreateTeam = async () => {
    if (!newTeamName.trim()) {
      setMessage('Please enter a team name');
      return;
    }

    setLoading(true);
    setMessage('');
    try {
      const token = localStorage.getItem('token');
      const response = await axios.post(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/teams/create`,
        { team_name: newTeamName },
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );

      // Show success message with credentials
      const msg = response.data.message || 'Team created successfully';
      const credentials = response.data.user_created
        ? `\n\nLogin Credentials:\nUsername: ${response.data.username}\nPassword: ${response.data.password}`
        : '';

      alert(msg + credentials);
      setMessage(msg);
      setNewTeamName('');
      fetchTeams(); // Refresh the teams list
    } catch (error) {
      console.error('Error creating team:', error);
      const errorMsg = error.response?.data?.message || 'Failed to create team';
      alert(errorMsg);
      setMessage(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  const handleRefreshQuestionCache = async () => {
    setLoading(true);
    setMessage('');
    try {
      const token = localStorage.getItem('token');
      const response = await axios.get(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/problems/refresh-cache`,
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );
      setMessage(response.data.message || 'Cache refreshed successfully');
    } catch (error) {
      console.error('Error refreshing cache:', error);
      setMessage('Failed to refresh cache');
    } finally {
      setLoading(false);
    }
  };

  const handleGameControl = async (action) => {
    setLoading(true);
    setMessage('');
    try {
      const token = localStorage.getItem('token');
      const response = await axios.post(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/game/control`,
        { action },
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );
      setMessage(response.data.message || `Game ${action}ed successfully`);
      setGameStatus(action);
    } catch (error) {
      console.error('Error controlling game:', error);
      setMessage(`Failed to ${action} game`);
    } finally {
      setLoading(false);
    }
  };

  const addTestCase = () => {
    if (!testCaseForm.name || !testCaseForm.input || !testCaseForm.expected_output) {
      setMessage('Please fill all test case fields');
      return;
    }

    setTestCases([...testCases, { ...testCaseForm }]);
    setTestCaseForm({ name: '', input: '', expected_output: '', type: 'visible' });
    setMessage('Test case added');
  };

  const removeTestCase = (index) => {
    setTestCases(testCases.filter((_, i) => i !== index));
  };

  const handleUploadProblem = async () => {
    if (!problemForm.id || !problemForm.title || !problemForm.description) {
      setMessage('Please fill all problem fields');
      return;
    }

    if (testCases.length === 0) {
      setMessage('Please add at least one test case');
      return;
    }

    setLoading(true);
    setMessage('');
    try {
      const token = localStorage.getItem('token');

      // Convert test cases array to object
      const testCasesObj = {};
      testCases.forEach((tc) => {
        testCasesObj[tc.name] = {
          input: tc.input,
          expected_output: tc.expected_output,
          type: tc.type
        };
      });

      const response = await axios.post(
        `${process.env.REACT_APP_SERVER_BASEAPI}/admin/problems/upload`,
        {
          ...problemForm,
          test_cases: testCasesObj
        },
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );

      setMessage(response.data.message || 'Problem uploaded successfully');

      // Reset form
      setProblemForm({
        id: '',
        title: '',
        description: '',
        difficulty: 'easy',
        points: 10,
        test_cases: {}
      });
      setTestCases([]);
    } catch (error) {
      console.error('Error uploading problem:', error);
      setMessage(error.response?.data?.message || 'Failed to upload problem');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="admin-utils-container">
      <h1 className="admin-utils-title">Admin Control Panel</h1>

      {/* Game Status Section */}
      <div className="admin-section">
        <h2>Game Status</h2>
        <div className="status-info">
          <p><strong>Current Status:</strong> <span className={`status-badge ${gameStatus}`}>{gameStatus.toUpperCase()}</span></p>
          <p><strong>Total Teams:</strong> {totalTeams}</p>
        </div>
        <div className="button-group">
          <button
            className="admin-button start"
            onClick={() => handleGameControl('start')}
            disabled={loading || gameStatus === 'start'}
          >
            Start Game
          </button>
          <button
            className="admin-button pause"
            onClick={() => handleGameControl('pause')}
            disabled={loading || gameStatus === 'pause'}
          >
            Pause Game
          </button>
          <button
            className="admin-button stop"
            onClick={() => handleGameControl('stop')}
            disabled={loading || gameStatus === 'stop'}
          >
            Stop Game
          </button>
          <button
            className="admin-button reset"
            onClick={() => handleGameControl('reset')}
            disabled={loading}
          >
            Reset Game
          </button>
        </div>
      </div>

      {/* Participating Teams Section */}
      <div className="admin-section">
        <h2>Participating Teams ({teams.length})</h2>

        {/* Create Team Form */}
        <div className="create-team-form">
          <h3>Create New Team</h3>
          <div className="form-row">
            <div className="form-group" style={{flex: 1}}>
              <input
                type="text"
                value={newTeamName}
                onChange={(e) => setNewTeamName(e.target.value)}
                placeholder="Enter team name"
                disabled={loading}
              />
            </div>
            <button
              className="admin-button add-test"
              onClick={handleCreateTeam}
              disabled={loading || !newTeamName.trim()}
              style={{marginTop: 0}}
            >
              Create Team
            </button>
          </div>
        </div>

        {teams.length === 0 ? (
          <p className="no-data">No teams registered yet</p>
        ) : (
          <div className="teams-table-container">
            <table className="teams-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>Team Name</th>
                  <th>Points</th>
                  <th>Coins</th>
                  <th>Created At</th>
                </tr>
              </thead>
              <tbody>
                {teams.map((team, index) => (
                  <tr key={team.id || index}>
                    <td>{index + 1}</td>
                    <td className="team-name">{team.name}</td>
                    <td className="points">{team.points || 0}</td>
                    <td className="coins">{team.coins || 0}</td>
                    <td className="date">
                      {team.created_at ? new Date(team.created_at).toLocaleDateString() : 'N/A'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <button
          className="admin-button refresh"
          onClick={fetchTeams}
          disabled={loading}
        >
          Refresh Teams
        </button>
      </div>

      {/* Uploaded Challenges Section */}
      <div className="admin-section">
        <h2>Uploaded Challenges ({challenges.length})</h2>

        {challenges.length === 0 ? (
          <p className="no-data">No challenges uploaded yet</p>
        ) : (
          <div className="teams-table-container">
            <table className="teams-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>ID</th>
                  <th>Title</th>
                  <th>Difficulty</th>
                  <th>Points</th>
                  <th>Test Cases</th>
                  <th>Created At</th>
                </tr>
              </thead>
              <tbody>
                {challenges.map((challenge, index) => (
                  <tr key={challenge.id || index}>
                    <td>{index + 1}</td>
                    <td className="challenge-id">{challenge.id}</td>
                    <td className="team-name">{challenge.title}</td>
                    <td>
                      <span className={`difficulty-badge ${challenge.difficulty}`}>
                        {challenge.difficulty}
                      </span>
                    </td>
                    <td className="points">{challenge.points || 0}</td>
                    <td className="coins">{challenge.test_cases?.length || 0}</td>
                    <td className="date">
                      {challenge.created_at ? new Date(challenge.created_at).toLocaleDateString() : 'N/A'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <button
          className="admin-button refresh"
          onClick={fetchChallenges}
          disabled={loading}
        >
          Refresh Challenges
        </button>
      </div>

      {/* Cache Management Section */}
      <div className="admin-section">
        <h2>Cache Management</h2>
        <button
          className="admin-button refresh"
          onClick={handleRefreshQuestionCache}
          disabled={loading}
        >
          {loading ? 'Refreshing...' : 'Refresh Question Cache'}
        </button>
      </div>

      {/* Problem Upload Section */}
      <div className="admin-section">
        <h2>Upload New Problem</h2>
        <div className="form-group">
          <label>Problem ID:</label>
          <input
            type="text"
            value={problemForm.id}
            onChange={(e) => setProblemForm({ ...problemForm, id: e.target.value })}
            placeholder="e.g., q1, q2, q3"
          />
        </div>
        <div className="form-group">
          <label>Title:</label>
          <input
            type="text"
            value={problemForm.title}
            onChange={(e) => setProblemForm({ ...problemForm, title: e.target.value })}
            placeholder="Problem title"
          />
        </div>
        <div className="form-group">
          <label>Description:</label>
          <textarea
            value={problemForm.description}
            onChange={(e) => setProblemForm({ ...problemForm, description: e.target.value })}
            placeholder="Problem description (supports markdown)"
            rows="6"
          />
        </div>
        <div className="form-row">
          <div className="form-group">
            <label>Difficulty:</label>
            <select
              value={problemForm.difficulty}
              onChange={(e) => setProblemForm({ ...problemForm, difficulty: e.target.value })}
            >
              <option value="easy">Easy</option>
              <option value="medium">Medium</option>
              <option value="hard">Hard</option>
            </select>
          </div>
          <div className="form-group">
            <label>Points:</label>
            <select
              value={problemForm.points}
              onChange={(e) => setProblemForm({ ...problemForm, points: parseInt(e.target.value) })}
            >
              <option value="10">10 (Easy)</option>
              <option value="20">20 (Medium)</option>
              <option value="30">30 (Hard)</option>
            </select>
          </div>
        </div>

        {/* Test Cases Section */}
        <div className="test-cases-section">
          <h3>Test Cases</h3>
          <div className="test-case-form">
            <div className="form-group">
              <label>Test Case Name:</label>
              <input
                type="text"
                value={testCaseForm.name}
                onChange={(e) => setTestCaseForm({ ...testCaseForm, name: e.target.value })}
                placeholder="e.g., test1, test2"
              />
            </div>
            <div className="form-group">
              <label>Input:</label>
              <textarea
                value={testCaseForm.input}
                onChange={(e) => setTestCaseForm({ ...testCaseForm, input: e.target.value })}
                placeholder="Test case input"
                rows="3"
              />
            </div>
            <div className="form-group">
              <label>Expected Output:</label>
              <textarea
                value={testCaseForm.expected_output}
                onChange={(e) => setTestCaseForm({ ...testCaseForm, expected_output: e.target.value })}
                placeholder="Expected output"
                rows="3"
              />
            </div>
            <div className="form-group">
              <label>Type:</label>
              <select
                value={testCaseForm.type}
                onChange={(e) => setTestCaseForm({ ...testCaseForm, type: e.target.value })}
              >
                <option value="visible">Visible</option>
                <option value="hidden">Hidden</option>
              </select>
            </div>
            <button className="admin-button add-test" onClick={addTestCase}>
              Add Test Case
            </button>
          </div>

          {/* Display Added Test Cases */}
          {testCases.length > 0 && (
            <div className="test-cases-list">
              <h4>Added Test Cases ({testCases.length}):</h4>
              {testCases.map((tc, index) => (
                <div key={index} className="test-case-item">
                  <div className="test-case-header">
                    <strong>{tc.name}</strong>
                    <span className={`badge ${tc.type}`}>{tc.type}</span>
                    <button
                      className="remove-btn"
                      onClick={() => removeTestCase(index)}
                    >
                      ×
                    </button>
                  </div>
                  <div className="test-case-content">
                    <p><strong>Input:</strong> {tc.input.substring(0, 50)}{tc.input.length > 50 ? '...' : ''}</p>
                    <p><strong>Output:</strong> {tc.expected_output.substring(0, 50)}{tc.expected_output.length > 50 ? '...' : ''}</p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <button
          className="admin-button upload"
          onClick={handleUploadProblem}
          disabled={loading}
        >
          {loading ? 'Uploading...' : 'Upload Problem'}
        </button>
      </div>

      {/* Message Display */}
      {message && (
        <div className={`admin-message ${message.includes('Failed') || message.includes('error') ? 'error' : 'success'}`}>
          {message}
        </div>
      )}
    </div>
  );
};

export default AdminUtils;
