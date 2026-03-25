package database

import (
	"code-mafia-backend/internal/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository struct {
	db *mongo.Database
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{db: db}
}

// User operations
func (r *Repository) GetUserByUsername(username string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	collection := r.db.Collection("users")
	err := collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateUser(username, hashedPassword string, teamID string, role string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user := &models.User{
		Username:  username,
		Password:  hashedPassword,
		TeamID:    teamID,
		Role:      role,
		CreatedAt: time.Now(),
	}

	collection := r.db.Collection("users")
	result, err := collection.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	user.ID = result.InsertedID.(bson.ObjectID).Hex()
	return user, nil
}

// Team operations
func (r *Repository) GetTeamByID(teamID string) (*models.Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var team models.Team
	collection := r.db.Collection("teams")

	objectID, err := bson.ObjectIDFromHex(teamID)
	if err != nil {
		return nil, err
	}

	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&team)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *Repository) GetTeamByName(name string) (*models.Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var team models.Team
	collection := r.db.Collection("teams")
	err := collection.FindOne(ctx, bson.M{"name": name}).Decode(&team)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *Repository) CreateTeam(name string) (*models.Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	team := &models.Team{
		Name:      name,
		Points:    0,
		Coins:     50,
		CreatedAt: time.Now(),
	}

	collection := r.db.Collection("teams")
	result, err := collection.InsertOne(ctx, team)
	if err != nil {
		return nil, err
	}

	team.ID = result.InsertedID.(bson.ObjectID).Hex()
	return team, nil
}

func (r *Repository) UpdateTeamPoints(teamID string, points int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectID, err := bson.ObjectIDFromHex(teamID)
	if err != nil {
		return err
	}

	collection := r.db.Collection("teams")
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{"points": points}},
	)
	return err
}

func (r *Repository) UpdateTeamCoins(teamID string, coins int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectID, err := bson.ObjectIDFromHex(teamID)
	if err != nil {
		return err
	}

	collection := r.db.Collection("teams")
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{"coins": coins}},
	)
	return err
}

func (r *Repository) UpdateTeamPointsAndCoins(teamID string, points, coins int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectID, err := bson.ObjectIDFromHex(teamID)
	if err != nil {
		return err
	}

	collection := r.db.Collection("teams")
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{"points": points, "coins": coins}},
	)
	return err
}

func (r *Repository) GetAllTeams() ([]models.Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := r.db.Collection("teams")
	opts := options.Find().SetSort(bson.D{{Key: "points", Value: -1}, {Key: "coins", Value: -1}})
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var teams []models.Team
	if err = cursor.All(ctx, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// Challenge operations
func (r *Repository) GetAllChallenges() ([]models.Challenge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := r.db.Collection("challenges")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var challenges []models.Challenge
	if err = cursor.All(ctx, &challenges); err != nil {
		return nil, err
	}
	return challenges, nil
}

func (r *Repository) GetChallengeByID(id string) (*models.Challenge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var challenge models.Challenge
	collection := r.db.Collection("challenges")

	// The ID could be a valid ObjectID hex string or a regular string UUID
	var query bson.M
	objectID, err := bson.ObjectIDFromHex(id)
	if err == nil {
		query = bson.M{"_id": objectID}
	} else {
		query = bson.M{"_id": id}
	}

	err = collection.FindOne(ctx, query).Decode(&challenge)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &challenge, nil
}

func (r *Repository) CreateChallenge(id, title, description, difficulty string, points int, testCases []models.TestCase, starterCode map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	challenge := &models.Challenge{
		ID:          id,
		Title:       title,
		Description: description,
		Difficulty:  difficulty,
		Points:      points,
		TestCases:   testCases,
		StarterCode: starterCode,
		CreatedAt:   time.Now(),
	}

	collection := r.db.Collection("challenges")
	_, err := collection.InsertOne(ctx, challenge)
	return err
}

func (r *Repository) DeleteChallengeByID(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := r.db.Collection("challenges")

	var query bson.M
	objectID, err := bson.ObjectIDFromHex(id)
	if err == nil {
		query = bson.M{"_id": objectID}
	} else {
		query = bson.M{"_id": id}
	}

	_, err = collection.DeleteOne(ctx, query)
	return err
}

// Submission operations
func (r *Repository) GetSubmissionByTeamAndChallenge(teamID string, challengeID string) (*models.Submission, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var submission models.Submission
	collection := r.db.Collection("submissions")
	err := collection.FindOne(ctx, bson.M{
		"team_id":      teamID,
		"challenge_id": challengeID,
	}).Decode(&submission)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &submission, nil
}

func (r *Repository) CreateSubmission(teamID string, challengeID, code, status string, pointsAwarded int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	submission := &models.Submission{
		TeamID:        teamID,
		ChallengeID:   challengeID,
		Code:          code,
		Status:        status,
		PointsAwarded: pointsAwarded,
		SubmittedAt:   time.Now(),
	}

	collection := r.db.Collection("submissions")
	_, err := collection.InsertOne(ctx, submission)
	return err
}

func (r *Repository) UpdateSubmission(id string, code, status string, pointsAwarded int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	collection := r.db.Collection("submissions")
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{
			"code":           code,
			"status":         status,
			"points_awarded": pointsAwarded,
		}},
	)
	return err
}

func (r *Repository) GetSubmissionsByTeam(teamID string) ([]models.Submission, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := r.db.Collection("submissions")
	cursor, err := collection.Find(ctx, bson.M{"team_id": teamID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var submissions []models.Submission
	if err = cursor.All(ctx, &submissions); err != nil {
		return nil, err
	}
	return submissions, nil
}

// PowerUp operations
func (r *Repository) GetAllPowerUps() ([]models.PowerUp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := r.db.Collection("power_ups")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var powerUps []models.PowerUp
	if err = cursor.All(ctx, &powerUps); err != nil {
		return nil, err
	}
	return powerUps, nil
}
